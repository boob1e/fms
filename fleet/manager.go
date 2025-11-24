package fleet

import (
	"context"
	"fmt"
	"log"
	"sync"
)

type DeviceManager struct {
	devices map[string]Worker
	cancels map[string]context.CancelFunc
	mu      sync.RWMutex
	appCtx  context.Context
	broker  Broker
}

func NewDeviceManager(appCtx context.Context, broker Broker) *DeviceManager {
	return &DeviceManager{
		devices: make(map[string]Worker),
		cancels: make(map[string]context.CancelFunc),
		appCtx:  appCtx,
		broker:  broker,
	}
}

func (m *DeviceManager) Register(worker Worker, zone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := worker.GetID().String()
	if _, exists := m.devices[key]; exists {
		return fmt.Errorf("device for key %s already exists", key)
	}
	ctx, cancel := context.WithCancel(m.appCtx)

	worker.Start(ctx)

	m.devices[key] = worker
	m.cancels[key] = cancel

	return nil
}

func (m *DeviceManager) Unregister(worker Worker) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := worker.GetID().String()
	if _, exists := m.devices[key]; !exists {
		return fmt.Errorf("worker with id %s not found", key)
	}
	if cancel, exists := m.cancels[key]; exists {
		cancel() // Signal context cancellation
	}
	worker.Shutdown()
	delete(m.devices, key)
	delete(m.cancels, key)
	return nil
}

func (m *DeviceManager) UnregisterByWorkerId(uid string) error {

	w, found := m.devices[uid]
	if !found {
		return fmt.Errorf("failed to unregister worker with id %s", uid)
	}

	err := m.Unregister(w)
	if err != nil {
		return err
	}
	log.Printf("unregistered device %s", uid)
	return nil
}

func (m *DeviceManager) Shutdown() {
	// TODO: shutdown all devices
}

func (m *DeviceManager) ShutdownAll() {
	for _, device := range m.devices {
		device.Shutdown()
	}
}
