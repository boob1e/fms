// Package fleet is for managing iot fleet and handling messages between them
package fleet

import (
	"context"
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
	ID      uuid.UUID
	broker  Broker // Interface for testability and flexibility
	inbox   chan Task
	topic   string             // Subscription topic for cleanup
	ctx     context.Context    // Device lifecycle context
	cancel  context.CancelFunc // Cancel function for shutdown
	wg      sync.WaitGroup
	handler TaskHandler // Injected device-specific task handler - strategy pattern gang of four
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
	d.broker.Unsubscribe(d.topic, d.inbox)
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
