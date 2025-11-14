package fleet

import (
	"context"

	"gorm.io/gorm"
)

type RegisterDeviceReq struct {
}

//TODO: deviceRegistrar interface?

type DeviceService struct {
	registry *DeviceManager
	broker   Broker
	db       *gorm.DB
	appCtx   context.Context
}

func NewDeviceService(r *DeviceManager, b Broker, db *gorm.DB, appCtx context.Context) *DeviceService {
	return &DeviceService{
		registry: r,
		broker:   b,
		db:       db,
		appCtx:   appCtx,
	}
}

func (s *DeviceService) RegisterDevice(req RegisterDeviceReq) error {
	//TODO: switch based on device type?

	deviceType := "test"
	switch deviceType {
	case "sprinkler":
		sprinkler := NewSprinkler(, s.broker, "zoneA")
		s.registry.Register(sprinkler, "zoneA")
	}
	//take request and create a RegisterDevice(
	// insert device into table
	// should now have id, register device to registry(manager)

	// return if it was registered or not
	return nil
}
