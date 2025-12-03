package fleet

import (
	"context"
	"errors"
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
	Shutdown()
}

type DLQManager interface {
	GetDLQTasks() []DLQEntry
	RequeueFromDLQ(taskID uuid.UUID) error
	RemoveFromDLQ(taskID uuid.UUID) error
	ClearDLQ()
}

type MessageBroker struct {
	subscribers map[string][]Subscriber // keys are topics
	ackChan     chan TaskAck
	taskStates  map[uuid.UUID]*TaskState
	retryQueue  map[uuid.UUID]*TaskRetryState
	retryConfig RetryConfig
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	dlq         []DLQEntry
	dlqMu       sync.RWMutex
}

func NewMessageBroker() *MessageBroker {
	ctx, cancel := context.WithCancel(context.Background())
	b := &MessageBroker{
		subscribers: make(map[string][]Subscriber),
		ackChan:     make(chan TaskAck, 100),
		taskStates:  make(map[uuid.UUID]*TaskState),
		retryQueue:  make(map[uuid.UUID]*TaskRetryState),
		retryConfig: DefaultRetryConfig(),
		ctx:         ctx,
		cancel:      cancel,
	}

	b.wg.Add(2)
	go b.listenForACKs()
	go b.initRetryBackgroundProcess()

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

	if len(subs) == 0 {
		log.Printf("[WARN] No subscribers for topic '%s', task %s not delivered", task.Topic, task.ID)
		return nil
	}

	var (
		successCount     int
		timeoutCount     int
		channelFullCount int
	)

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

func (b *MessageBroker) listenForACKs() {
	defer b.wg.Done()
	for {
		select {
		case <-b.ctx.Done():
			return
		case ack, ok := <-b.ackChan:
			if !ok {
				return
			}
			b.processACK(ack)
		}
	}
}

func (b *MessageBroker) processACK(ack TaskAck) {

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
					entry := DLQEntry{
						Task:          t.Task,
						FailureReason: "attempts exceeded",
						Attempts:      t.Attempts,
						LastError:     t.LastError,
						AddedAt:       time.Now(),
					}
					b.dlqMu.Lock()
					b.dlq = append(b.dlq, entry)
					b.dlqMu.Unlock()
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

func (b *MessageBroker) Shutdown() {
	// Signal all goroutines to stop
	b.cancel()

	// Wait for all goroutines to finish
	b.wg.Wait()

	// Close the ACK channel to prevent further writes
	close(b.ackChan)
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

func (b *MessageBroker) initRetryBackgroundProcess() {
	defer b.wg.Done()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-time.After(1 * time.Second):
			b.mu.Lock()
			var tasksToRetry []Task

			for _, t := range b.retryQueue {
				if t.NextRetry.After(time.Now()) {
					continue
				}
				tasksToRetry = append(tasksToRetry, t.Task)
			}
			b.mu.Unlock()

			for _, task := range tasksToRetry {
				ctx := context.Background()
				b.Publish(ctx, task)
				log.Printf("[RETRY] Republishing task %s", task.ID)
			}
		}
	}
}

func (b *MessageBroker) GetDLQTasks() []DLQEntry {
	b.dlqMu.RLock()
	defer b.dlqMu.RUnlock()
	cpy := make([]DLQEntry, len(b.dlq))
	copy(cpy, b.dlq)
	return cpy
}

func (b *MessageBroker) RequeueFromDLQ(taskID uuid.UUID) error {
	b.dlqMu.Lock()

	var found bool
	var task Task
	var lastError string
	for i, value := range b.dlq {
		if value.Task.ID == taskID {
			task = value.Task
			lastError = value.LastError
			b.dlq = append(b.dlq[:i], b.dlq[i+1:]...)
			found = true
			break
		}
	}
	b.dlqMu.Unlock()

	if !found {
		return errors.New("task not found in DLQ")
	}

	b.mu.Lock()
	b.retryQueue[taskID] = &TaskRetryState{
		Task:        task,
		Attempts:    0,
		MaxRetries:  b.retryConfig.MaxRetries,
		LastAttempt: time.Now(),
		NextRetry:   time.Now(),
		LastError:   lastError,
	}
	b.mu.Unlock()

	return nil
}

func (b *MessageBroker) RemoveFromDLQ(taskID uuid.UUID) error {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()
	for i, value := range b.dlq {
		if value.Task.ID == taskID {
			b.dlq = append(b.dlq[:i], b.dlq[i+1:]...)
			return nil
		}
	}
	return errors.New("task not found to remove")
}

func (b *MessageBroker) ClearDLQ() {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()
	b.dlq = nil
}
