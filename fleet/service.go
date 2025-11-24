package fleet

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
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
	manager *DeviceManager
	broker  Broker
	db      *gorm.DB
	appCtx  context.Context
}

func NewDeviceService(m *DeviceManager, b Broker, db *gorm.DB, appCtx context.Context) *DeviceService {
	return &DeviceService{
		manager: m,
		broker:  b,
		db:      db,
		appCtx:  appCtx,
	}
}

func (s *DeviceService) RegisterDevice(req RegisterDeviceReq) (string, error) {
	switch req.DeviceType {
	case "sprinkler":
		sprinkler := NewSprinkler(s.broker, req.Zone)
		err := s.manager.Register(sprinkler, req.Zone)
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
			s.manager.Unregister(sprinkler)
			return "", err
		}
		log.Printf("successfully created device: %s", d.Device.UUID)
		return d.Device.UUID.String(), nil
	default:
		log.Println("device type not recognized")
		return "", fmt.Errorf("unsupported device type: %s", req.DeviceType)
	}
}

func (s *DeviceService) UnregisterDevice(deviceID uuid.UUID) error {
	err := s.manager.UnregisterByWorkerId(deviceID.String())
	if err != nil {
		return err
	}

	rowsDeleted, err := gorm.G[RegisteredDevice](s.db).Where("UUID = ?", deviceID).Delete(s.appCtx)
	if err != nil {
		log.Printf("device %s unregistered but failed to remove from db", deviceID)
		return err
	}
	log.Printf("worker and underlying device types deleted from db, total rows: %d", rowsDeleted)
	return nil
}

func (s *DeviceService) ProcessTask(task Task) error {
	err := s.broker.Publish(s.appCtx, task)
	if err != nil {
		return err
	}
	return nil
}
