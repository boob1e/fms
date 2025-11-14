package main

import (
	"context"
	"farm_management_system/database"
	"farm_management_system/fleet"
	"log"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

type FMS struct {
	broker  fleet.Broker
	db      *gorm.DB
	manager *fleet.Manager
}

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	if err := database.AutoMigrate(db); err != nil {
		log.Fatal("Failed to run migrations:", err)
	}

	broker := fleet.NewMessageBroker()

	manager := &fleet.Manager{}
	fms := &FMS{
		broker:  broker,
		db:      db,
		manager: manager,
	}
	app := fiber.New(fiber.Config{
		AppName: "FMS v1.0",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// api := app.Group("/api")

	registerDevices(fms)
	log.Fatal(app.Listen(":3000"))
}

func registerDevices(fms *FMS) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sprinklerZoneA := fleet.NewSprinkler(ctx, fms.broker, "a")
	fms.manager.RegisterDevice(sprinklerZoneA)
}
