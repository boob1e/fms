// Package fleet is for managing iot fleet and handling messages between them
package fleet

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type Device interface {
	TaskHandler
	GetID() uuid.UUID
	Start(ctx context.Context)
	Shutdown()
}

type device struct {
	ID      uuid.UUID
	broker  Broker // Interface for testability and flexibility
	inbox   chan Task
	wg      sync.WaitGroup
	handler TaskHandler // Injected device-specific task handler - strategy pattern gang of four
}

func (d *device) GetID() uuid.UUID {
	return d.ID
}
func (d *device) Start(ctx context.Context) {
	d.wg.Add(1)
	go d.listen(ctx)
}

func (d *device) Shutdown() {
	d.broker.Unsubscribe("irrigation-zone", d.inbox)
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

type DeviceManager struct {
	devices map[string]Device
	cancels map[string]context.CancelFunc
	mu      sync.RWMutex
	appCtx  context.Context
	broker  Broker
}

func NewDeviceManager(appCtx context.Context, broker Broker) *DeviceManager {
	return &DeviceManager{
		devices: make(map[string]Device),
		cancels: make(map[string]context.CancelFunc),
		appCtx:  appCtx,
		broker:  broker,
	}
}

func (m *DeviceManager) Register(device Device, zone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[zone]; exists {
		return fmt.Errorf("device for zone %s already exists", zone)
	}
	ctx, cancel := context.WithCancel(m.appCtx)

	device.Start(ctx)

	key := device.GetID().String()
	m.devices[key] = device
	m.cancels[key] = cancel

	return nil
}

func (m *DeviceManager) Unregister() {

}

func (m *DeviceManager) Shutdown() {
	// TODO: shutdown all devices
}

func (m *DeviceManager) ShutdownAll() {
	for _, device := range m.devices {
		device.Shutdown()
	}
}
