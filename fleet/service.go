package fleet

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"
)

type RegisterDeviceReq struct {
	Name         string
	SerialNumber string
	WorkerType   WorkerType
	Zone         string
	DeviceType   DeviceType
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

func (s *DeviceService) RegisterDevice(req RegisterDeviceReq) (string, error) {
	switch req.DeviceType {
	case "sprinkler":
		sprinkler := NewSprinkler(s.broker, req.Zone)
		err := s.registry.Register(sprinkler, req.Zone)
		if err != nil {
			return "", err
		}
		d := &IrrigationDevice{
			Device: RegisteredDevice{
				Name:         req.Name,
				SerialNumber: req.SerialNumber,
				Status:       DeviceStatusAvailable,
				WorkerType:   req.WorkerType,
				DeviceType:   req.DeviceType,
				Zone:         req.Zone,
				UUID:         sprinkler.GetID(),
			},
		}
		err = gorm.G[IrrigationDevice](s.db).Create(s.appCtx, d)
		if err != nil {
			s.registry.Unregister(sprinkler)
			return "", err
		}
		log.Printf("successfully created device: %s", d.Device.UUID)
		return d.Device.UUID.String(), nil
	default:
		log.Println("device type not recognized")
		return "", fmt.Errorf("unsupported device type: %s", req.DeviceType)
	}
}
