package fleet

import (
	"log"
	"sync"

	"github.com/google/uuid"
)

type Subscriber chan Task

type Broker interface {
	Subscribe(topic string) Subscriber
	Unsubscribe(topic string, ch Subscriber)
	Publish(topic string, task Task)
}

type MessageBroker struct {
	subscribers map[string][]Subscriber // keys are topics
	mu          sync.RWMutex
}

func NewBroker() *MessageBroker {
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

	close(ch)

	log.Println("todo: Unsubscribe")
}

func (b *MessageBroker) Publish(topic string, task Task) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subscribers[topic] {
		select {
		case sub <- task:

			log.Println("received a message to publish")
		default:
		}
	}
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
