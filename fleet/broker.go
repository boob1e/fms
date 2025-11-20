package fleet

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Subscriber chan Task

type Broker interface {
	Subscribe(topic string) Subscriber
	Unsubscribe(topic string, ch Subscriber)
	Publish(ctx context.Context, topic string, task Task) error
	GetACKChannel() chan TaskAck
	GetTaskStatus(taskID uuid.UUID) (*TaskState, error)
}

type MessageBroker struct {
	subscribers map[string][]Subscriber // keys are topics
	ackChan     chan TaskAck
	taskStates  map[uuid.UUID]*TaskState
	mu          sync.RWMutex
}

func NewMessageBroker() *MessageBroker {
	b := &MessageBroker{
		subscribers: make(map[string][]Subscriber),
		ackChan:     make(chan TaskAck, 100),
		taskStates:  make(map[uuid.UUID]*TaskState),
	}
	go b.processACKs()

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

func (b *MessageBroker) Publish(ctx context.Context, topic string, task Task) error {
	// Initialize task state tracking
	b.mu.Lock()
	b.taskStates[task.ID] = &TaskState{
		Task:        task,
		PublishedAt: time.Now(),
		Status:      Queued,
	}
	b.mu.Unlock()

	b.mu.RLock()
	subs := b.subscribers[topic]
	b.mu.RUnlock()

	// No subscribers is not an error, just a no-op
	if len(subs) == 0 {
		log.Printf("[WARN] No subscribers for topic '%s', task %s not delivered", topic, task.ID)
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
			log.Printf("[DEBUG] Task %s delivered to subscriber %d on topic '%s'", task.ID, i, topic)
		case <-ctx.Done():
			timeoutCount++
			log.Printf("[ERROR] Task %s failed to subscriber %d on topic '%s': context timeout/cancelled",
				task.ID, i, topic)
		default:
			channelFullCount++
			log.Printf("[ERROR] Task %s failed to subscriber %d on topic '%s': channel full",
				task.ID, i, topic)
		}
	}

	// Return error if ANY delivery failed, but include success stats
	if timeoutCount > 0 || channelFullCount > 0 {
		return fmt.Errorf(
			"partial delivery failure on topic '%s': %d/%d delivered (%d timeout, %d channel full)",
			topic, successCount, len(subs), timeoutCount, channelFullCount,
		)
	}

	log.Printf("[INFO] Task %s successfully delivered to all %d subscribers on topic '%s'",
		task.ID, successCount, topic)
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
			case Complete, Failed:
				if state.CompletedAt == nil {
					now := ack.Timestamp
					state.CompletedAt = &now
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
