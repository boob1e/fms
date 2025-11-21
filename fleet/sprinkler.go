package fleet

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Irrigator interface {
	StartWater(ctx context.Context) error
	StopWater()
}

type Sprinkler struct {
	device
	IsActive        bool
	PressureReading float32
	// TODO: currentCycleTime
	cancelWatering context.CancelFunc
	mu             sync.Mutex
}

func NewSprinkler(broker Broker, zone string) *Sprinkler {
	topic := "irrigation-" + zone
	ch := broker.Subscribe(topic)

	s := &Sprinkler{
		device: device{
			ID:     uuid.New(),
			broker: broker,
			inbox:  ch,
			topic:  topic,
		},
		IsActive:        false,
		PressureReading: 0,
	}

	// Inject self as the task handler (polymorphism via interface)
	s.handler = s

	return s
}

// HandleTask implements the TaskHandler interface for Sprinkler.
// It processes incoming tasks and delegates to device-specific methods.
func (s *Sprinkler) HandleTask(task Task) {
	log.Printf("Sprinkler received task %s: %s", task.ID, task.Instruction)
	ackChan := s.broker.GetACKChannel()
	switch task.Instruction {
	case "start":
		s.handleStartTask(task, ackChan)
	case "stop":
		s.StopWater()
		ackChan <- NewTaskAck(task.ID, Complete, s.ID)
	default:
		log.Printf("Unknown instruction: %s", task.Instruction)
		ackChan <- NewErrTaskAck(task.ID, s.ID, "Unknown instruction type")
	}
}

func (s *Sprinkler) handleStartTask(task Task, ackChan chan TaskAck) {
	ackChan <- NewTaskAck(task.ID, Running, s.ID)

	// Use device context as parent - cancels automatically on Shutdown
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.cancelWatering = cancel
	s.mu.Unlock()

	done := make(chan error, 1)

	// Track StartWater goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := s.StartWater(ctx)
		done <- err
	}()

	// Track completion handler goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := <-done
		if err != nil && err != context.Canceled {
			// Don't ACK if shutdown cancelled the context
			log.Printf("Task %s failed: %v", task.ID, err)
			ackChan <- NewErrTaskAck(task.ID, s.ID, err.Error())
		} else if err == nil {
			log.Printf("Task %s completed", task.ID)
			ackChan <- NewTaskAck(task.ID, Complete, s.ID)
		}
		s.mu.Lock()
		s.cancelWatering = nil
		s.mu.Unlock()
	}()
}

func (s *Sprinkler) StartWater(ctx context.Context) error {
	log.Println("Starting water...")
	s.mu.Lock()
	s.IsActive = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.IsActive = false
		s.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			// Context was cancelled (stop was called)
			log.Println("Water task cancelled")
			return ctx.Err()

		case <-time.After(1 * time.Second):
			log.Println("watering crops")
		}
	}
}

func (s *Sprinkler) StopWater() {
	log.Println("Stopping water...")
	// s.IsActive = false
	s.mu.Lock()
	if s.cancelWatering != nil {
		s.cancelWatering()
	}
	s.mu.Unlock()
	// publish to irrigation topic, as well as manager's topic
	// s.broker.Publish()
}
