package main

import (
	"farm_management_system/database"
	"log"

	"github.com/gofiber/fiber/v3"

	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func main() {
	if err := database.AutoMigrate(); err != nil {
		log.Fatal("failed to run migrations", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "FMS v1.0",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/heatlh", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	//TODO: configure a broker for all devices
	log.Fatal(app.Listen(":3000"))
}
