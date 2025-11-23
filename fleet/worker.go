// Package fleet is for managing iot fleet and handling messages between them
package fleet

import (
	"context"
	"log"
	"sync"

	"github.com/google/uuid"
)

type Worker interface {
	TaskHandler
	GetID() uuid.UUID
	Start(ctx context.Context)
	Shutdown()
}

type WorkerType string

const (
	WorkerTypeIrrigation WorkerType = "Irrigation"
)

type DeviceType string

const (
	DeviceTypeSprinkler DeviceType = "Sprinkler"
)

type DeviceStatus string

const (
	DeviceStatusUnknown   DeviceStatus = "Unknown"
	DeviceStatusAvailable DeviceStatus = "Available"
	DeviceStatusBusy      DeviceStatus = "Busy"
)

type device struct {
	ID            uuid.UUID
	broker        Broker // Interface for testability and flexibility
	inbox         chan Task
	subscriptions map[string]Subscriber
	ctx           context.Context    // Device lifecycle context
	cancel        context.CancelFunc // Cancel function for shutdown
	wg            sync.WaitGroup
	handler       TaskHandler // Injected device-specific task handler - strategy pattern gang of four
}

func (d *device) GetID() uuid.UUID {
	return d.ID
}

func (d *device) Start(ctx context.Context) {
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.wg.Add(1)
	go d.listen(d.ctx)
}

func (d *device) Shutdown() {
	d.cancel()
	for topic, ch := range d.subscriptions {
		d.broker.Unsubscribe(topic, ch)
	}
	d.wg.Wait()
}

func (d *device) listen(ctx context.Context) {
	defer d.wg.Done()
	for {
		select {
		case task := <-d.inbox:
			if d.handler != nil {
				// call inheritor's method
				d.handler.HandleTask(task)
			}
		case <-ctx.Done():
			return //exit
		}
	}
}

// subscribe adds a topic subscription and starts a fan-in goroutine
// to forward tasks from the topic channel to the device's inbox
func (d *device) subscribe(topic string) {
	ch := d.broker.Subscribe(topic)
	d.subscriptions[topic] = ch

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			select {
			case task := <-ch:
				log.Printf("Device %s received task from topic '%s'", d.ID, topic)
				d.inbox <- task
			case <-d.ctx.Done():
				log.Printf("Device %s fan-in goroutine for topic '%s' shutting down", d.ID, topic)
				return
			}
		}
	}()
}
