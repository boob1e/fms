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
	broker fleet.Broker
	db     *gorm.DB
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
	fms := &FMS{
		broker: broker,
		db:     db,
	}
	app := fiber.New(fiber.Config{
		AppName: "FMS v1.0",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//TODO: this is an example, make this dynamic via some type of ioc container or shared mem
	// sprinklerZoneA := fleet.NewSprinkler(ctx, fms.Broker, "a")

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	api := app.Group("/api")

	//TODO: configure a broker for all devices
	log.Fatal(app.Listen(":3000"))
}
