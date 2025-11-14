// Package fleet is for managing iot fleet and handling messages between them
package fleet

import (
	"context"
	"log"
	"sync"
)

type Device interface {
	TaskHandler
	Shutdown()
}

type Manager struct {
	devices []Device
}

func (m *Manager) RegisterDevice(device Device) {
	m.devices = append(m.devices, device)
}

func (m *Manager) ShutdownAll() {
	for _, device := range m.devices {
		device.Shutdown()
	}
}

type TaskHandler interface {
	HandleTask(task Task) error
}

type FleetDevice struct {
	broker  Broker // Interface for testability and flexibility
	inbox   chan Task
	wg      sync.WaitGroup
	handler TaskHandler // Injected device-specific task handler - strategy pattern gang of four
}

func (f *FleetDevice) listen(ctx context.Context) {
	defer f.wg.Done()
	for {
		select {
		case task := <-f.inbox:
			if f.handler != nil {
				if err := f.handler.HandleTask(task); err != nil {
					log.Printf("Task %s failed: %v", task.ID, err)
				}
			}
		case <-ctx.Done():
			return //exit
		}
	}
}

func (f *FleetDevice) Shutdown() {
	f.broker.Unsubscribe("irrigation-zone", f.inbox)
	f.wg.Wait()
}
