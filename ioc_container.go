package main

import (
	"context"
	"farm_management_system/database"
	"farm_management_system/fleet"
	"fmt"

	"gorm.io/gorm"
)

// IOC container
type FMS struct {
	broker  fleet.Broker
	db      *gorm.DB
	manager *fleet.DeviceManager
	appCtx  context.Context
	config  *AppConfig
}

type AppConfig struct {
	AppName     string
	Port        int
	DBPath      string
	AutoMigrate bool
	Environment string
}

func DefaultConfig() *AppConfig {
	return &AppConfig{
		AppName:     "FMS",
		Port:        3000,
		DBPath:      "folo.db",
		AutoMigrate: true,
		Environment: "dev",
	}
}

type FMSOption func(*FMS) error

func NewFMS(ctx context.Context, opts ...FMSOption) (*FMS, error) {
	broker := fleet.NewMessageBroker()
	manager := fleet.NewDeviceManager(ctx, broker)
	fms := &FMS{
		broker:  broker,          // Default broker
		manager: manager,         // Default manager
		config:  DefaultConfig(), // Default config
		appCtx:  ctx,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(fms); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Initialize database if not provided
	if fms.db == nil {
		db, err := database.InitDatabase()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize database: %w", err)
		}
		fms.db = db
	}

	// Auto-migrate if enabled
	if fms.config.AutoMigrate {
		if err := database.AutoMigrate(fms.db); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	return fms, nil

}

func WithDatabase(db *gorm.DB) FMSOption {
	return func(fms *FMS) error {
		fms.db = db
		return nil
	}
}

func WithAppName(name string) FMSOption {
	return func(fms *FMS) error {
		fms.config.AppName = name
		return nil
	}
}

func WithPort(port int) FMSOption {
	return func(fms *FMS) error {
		fms.config.Port = port
		return nil
	}
}
