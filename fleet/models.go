package fleet

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RegisteredDevice struct {
	gorm.Model
	UUID         uuid.UUID `gorm:"type:uuid;uniqueIndex"`
	Name         string
	SerialNumber string
	Status       DeviceStatus
	WorkerType   WorkerType
	DeviceType   DeviceType
	Zone         string
}

func (d *RegisteredDevice) BeforeCreate(tx *gorm.DB) error {
	if d.UUID == uuid.Nil { // Only generate if not already set
		d.UUID = uuid.New()
	}
	return nil
}

type IrrigationDevice struct {
	gorm.Model
	RegisteredDeviceID int
	Device             RegisteredDevice
	GallonsPerMinute   float32
}
