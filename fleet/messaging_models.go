package fleet

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
)

type Subscriber chan Task

type Broker interface {
	Subscribe(topic string) Subscriber
	Unsubscribe(topic string, ch Subscriber)
	Publish(ctx context.Context, topic string, task Task) error
}

type MessageBroker struct {
	subscribers map[string][]Subscriber // keys are topics
	mu          sync.RWMutex
}

func NewMessageBroker() *MessageBroker {
	return &MessageBroker{
		subscribers: make(map[string][]Subscriber),
	}
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
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscribers[topic]

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

type TaskStatus string

const (
	Queued   = "Queued"
	Running  = "Running"
	Complete = "Complete"
	Failed   = "Failed"
)

type Task struct {
	ID          uuid.UUID
	Instruction string
	Status      TaskStatus
}
