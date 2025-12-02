package fleet

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Subscriber chan Task

type Broker interface {
	Subscribe(topic string) Subscriber
	Unsubscribe(topic string, ch Subscriber)
	Publish(ctx context.Context, task Task) error
	GetACKChannel() chan TaskAck
	GetTaskStatus(taskID uuid.UUID) (*TaskState, error)
}

type MessageBroker struct {
	subscribers map[string][]Subscriber // keys are topics
	ackChan     chan TaskAck
	taskStates  map[uuid.UUID]*TaskState
	retryQueue  map[uuid.UUID]*TaskRetryState
	retryConfig RetryConfig
	mu          sync.RWMutex
}

func NewMessageBroker() *MessageBroker {
	b := &MessageBroker{
		subscribers: make(map[string][]Subscriber),
		ackChan:     make(chan TaskAck, 100),
		taskStates:  make(map[uuid.UUID]*TaskState),
		retryQueue:  make(map[uuid.UUID]*TaskRetryState),
		retryConfig: DefaultRetryConfig(),
	}
	go b.processACKs()
	go b.processRetries()
	// TODO(human): Start processRetries() goroutine

	return b
}

func (b *MessageBroker) Subscribe(topic string) Subscriber {
	ch := make(Subscriber, 10)
	b.mu.Lock()
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	b.mu.Unlock()
	return ch
}

func (b *MessageBroker) Unsubscribe(topic string, ch Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[topic]
	for i, sub := range subs {
		if sub == ch {
			b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	log.Printf("Unsubscribed from topic '%s'", topic)

}

func (b *MessageBroker) Publish(ctx context.Context, task Task) error {
	// Initialize task state tracking
	b.mu.Lock()
	b.taskStates[task.ID] = &TaskState{
		Task:        task,
		PublishedAt: time.Now(),
		Status:      Queued,
	}
	b.mu.Unlock()

	b.mu.RLock()
	subs := b.subscribers[task.Topic]
	b.mu.RUnlock()

	// No subscribers is not an error, just a no-op
	if len(subs) == 0 {
		log.Printf("[WARN] No subscribers for topic '%s', task %s not delivered", task.Topic, task.ID)
		return nil
	}

	var (
		successCount     int
		timeoutCount     int
		channelFullCount int
	)

	// Best-effort: attempt delivery to ALL subscribers
	for i, sub := range subs {
		select {
		case sub <- task:
			successCount++
			log.Printf("[DEBUG] Task %s delivered to subscriber %d on topic '%s'", task.ID, i, task.Topic)
		case <-ctx.Done():
			timeoutCount++
			log.Printf("[ERROR] Task %s failed to subscriber %d on topic '%s': context timeout/cancelled",
				task.ID, i, task.Topic)
		default:
			channelFullCount++
			log.Printf("[ERROR] Task %s failed to subscriber %d on topic '%s': channel full",
				task.ID, i, task.Topic)
		}
	}

	// Return error if ANY delivery failed, but include success stats
	if timeoutCount > 0 || channelFullCount > 0 {
		return fmt.Errorf(
			"partial delivery failure on topic '%s': %d/%d delivered (%d timeout, %d channel full)",
			task.Topic, successCount, len(subs), timeoutCount, channelFullCount,
		)
	}

	log.Printf("[INFO] Task %s successfully delivered to all %d subscribers on topic '%s'",
		task.ID, successCount, task.Topic)
	return nil
}

func (b *MessageBroker) GetACKChannel() chan TaskAck {
	return b.ackChan
}

func (b *MessageBroker) processACKs() {
	for ack := range b.ackChan {
		log.Printf("[ACK] Task %s: %s (device: %s, time: %s)",
			ack.TaskID, ack.Status, ack.DeviceID, ack.Timestamp)

		if ack.Status == Failed && ack.Error != "" {
			log.Printf("[ACK] Error details: %s", ack.Error)
		}

		b.mu.Lock()
		if state, exists := b.taskStates[ack.TaskID]; exists {
			state.Status = ack.Status
			state.DeviceID = ack.DeviceID.String()

			switch ack.Status {
			case Running:
				if state.StartedAt == nil {
					now := ack.Timestamp
					state.StartedAt = &now
				}
			case Complete:
				if state.CompletedAt == nil {
					now := ack.Timestamp
					state.CompletedAt = &now
				}
				delete(b.retryQueue, ack.TaskID)
			case Failed:
				t, ok := b.retryQueue[ack.TaskID]
				if !ok {

					tr := &TaskRetryState{
						Attempts:    1,
						Task:        state.Task,
						MaxRetries:  b.retryConfig.MaxRetries,
						LastAttempt: time.Now(),
						LastError:   ack.Error,
					}
					backoff := calculateBackoff(tr.Attempts, b.retryConfig)
					tr.NextRetry = time.Now().Add(backoff)
					b.retryQueue[ack.TaskID] = tr
					log.Printf("[RETRY] Task %s failed, scheduling retry attempt %d in %s", ack.TaskID, tr.Attempts+1, tr.NextRetry.String())
				} else {
					t.Attempts++
					maxAttemptsExceeded := t.Attempts > b.retryConfig.MaxRetries
					if maxAttemptsExceeded {
						log.Printf("[RETRY] Task %s exceeded max retries (%d), giving up", ack.TaskID, b.retryConfig.MaxRetries)
						delete(b.retryQueue, ack.TaskID)
						break
					}
					backoff := calculateBackoff(t.Attempts, b.retryConfig)
					nextRetry := time.Now().Add(backoff)
					t.NextRetry = nextRetry
					log.Printf("[RETRY] Task %s failed, scheduling retry attempt %d in %s", ack.TaskID, t.Attempts+1, t.NextRetry.String())
				}
			}
		} else {
			log.Printf("ack for unknown task %s", ack.TaskID)
		}
		b.mu.Unlock()
	}

}

func (b *MessageBroker) GetTaskStatus(taskID uuid.UUID) (*TaskState, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ts, exists := b.taskStates[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	stateCopy := *ts
	return &stateCopy, nil
}

func calculateBackoff(attempts int, config RetryConfig) time.Duration {
	if attempts == 0 {
		return time.Second * 0
	}
	backoff := math.Pow(config.BackoffFactor, float64(attempts))
	backoff = backoff * float64(config.InitialBackoff)
	backoff = math.Min(backoff, float64(config.MaxBackoff))

	return time.Duration(backoff)
}

func (b *MessageBroker) processRetries() {
	for {
		time.Sleep(time.Second * 1)

		// Collect tasks to retry (while holding lock)
		b.mu.Lock()
		var tasksToRetry []Task
		var keysToDelete []uuid.UUID

		for key, t := range b.retryQueue {
			if t.NextRetry.After(time.Now()) {
				continue
			}
			tasksToRetry = append(tasksToRetry, t.Task)
			keysToDelete = append(keysToDelete, key)
		}
		b.mu.Unlock()

		// Publish tasks (without holding lock to avoid deadlock)
		for i, task := range tasksToRetry {
			ctx := context.Background()
			b.Publish(ctx, task)

			// Remove from retry queue after publish
			b.mu.Lock()
			delete(b.retryQueue, keysToDelete[i])
			b.mu.Unlock()

			log.Printf("[RETRY] Republishing task %s", task.ID)
		}
	}
}
