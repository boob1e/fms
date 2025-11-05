package fleet

import (
	"context"
	"log"
	"time"
)

type IrrigationDevice interface {
	StartWater()
	StopWater()
}

type Sprinkler struct {
	FleetDevice
	IsActive        bool
	PressureReading float32
	// TODO: currentCycleTime
}

func NewSprinkler(ctx context.Context, broker *MessageBroker, zone string) *Sprinkler {
	ch := broker.Subscribe("irrigation-" + zone)

	s := &Sprinkler{
		FleetDevice: FleetDevice{
			broker: broker,
			inbox:  ch,
		},
		IsActive:        false,
		PressureReading: 0,
	}

	s.wg.Add(1)
	go s.listen(ctx)

	return s
}

func (s *Sprinkler) StartWater() {
	s.IsActive = true
	for s.IsActive == true {
		time.Sleep(1 * time.Second)
		log.Println("watering crops")
	}
}

func (s *Sprinkler) StopWater() {
	s.IsActive = false
	// publish to irrigation topic, as well as manager's topic
	// s.broker.Publish()
}
