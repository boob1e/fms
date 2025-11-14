package main

import (
	"context"
	"farm_management_system/fleet"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v3"

	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func main() {
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	setupShutdownListener(appCancel)

	fms, err := NewFMS(
		WithAppName("FMS v1.0"),
		WithPort(":3000"),
		WithContext(appCtx),
	)

	if err != nil {
		log.Fatal("Failed to initialize FMS:", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "FMS v1.0",
	})

	mapRoutes(app)
	//TODO: this will run any stored devices in db on startup
	registerExistingDevices(fms)

	go func() {
		<-appCtx.Done()
		log.Println("Shutting down HTTP server...")
		app.Shutdown()
	}()

	log.Fatal(app.Listen(":3000"))
}

func setupShutdownListener(appCancel context.CancelFunc) {
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutdown signal received")
		appCancel()
	}()
}

func registerExistingDevices(fms *FMS) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fleet.NewSprinkler(ctx, fms.broker, "a")
	// fms.manager.RegisterDevice(sprinklerZoneA)
}

func mapRoutes(app *fiber.App) {
	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	api := app.Group("/api")

	fleetHandler := fleet.NewFleetHandler(nil)

	fleet.RegisterFleetRoutes(api, fleetHandler)
}
