package fleet

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type IrrigationDevice interface {
	StartWater(ctx context.Context) error
	StopWater()
}

type Sprinkler struct {
	FleetDevice
	IsActive        bool
	PressureReading float32
	// TODO: currentCycleTime
	cancelWatering context.CancelFunc
	mu             sync.Mutex
}

func NewSprinkler(ctx context.Context, broker Broker, zone string) *Sprinkler {
	ch := broker.Subscribe("irrigation-" + zone)

	s := &Sprinkler{
		FleetDevice: FleetDevice{
			broker: broker,
			inbox:  ch,
		},
		IsActive:        false,
		PressureReading: 0,
	}

	// Inject self as the task handler (polymorphism via interface)
	s.handler = s

	s.wg.Add(1)
	go s.listen(ctx)

	return s
}

// HandleTask implements the TaskHandler interface for Sprinkler.
// It processes incoming tasks and delegates to device-specific methods.
func (s *Sprinkler) HandleTask(task Task) error {
	log.Printf("Sprinkler received task %s: %s (Status: %s)", task.ID, task.Instruction, task.Status)

	switch task.Instruction {
	case "start":
		task.Status = Running

		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		s.cancelWatering = cancel
		s.mu.Unlock()

		done := make(chan error, 1)
		// non blocking task run
		go func() {
			err := s.StartWater(ctx)
			done <- err
		}()

		go func() {
			err := <-done
			if err != nil {
				task.Status = Failed
				log.Printf("Task %s failed: %v", task.ID, err)
			} else {
				task.Status = Complete
				log.Printf("Task %s completed", task.ID)
			}
			s.mu.Lock()
			s.cancelWatering = nil
			s.mu.Unlock()
		}()
	case "stop":
		s.StopWater()
		task.Status = Complete
	default:
		log.Printf("Unknown instruction: %s", task.Instruction)
		task.Status = Failed
		return errors.New("failed to handle sprinkler task")
	}

	return nil
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
