package fleet

import (
	"log"

	"github.com/gofiber/fiber/v3"
)

type FleetHandler struct {}

func NewFleetHandler() *FleetHandler {
	return &FleetHandler{}
}

func RegisterFleetRoutes(router fiber.Router) {
	// fleetDevices := router.Group("/fleet") 
}

// connects a new device to the network
func (h *FleetHandler) RegisterFleetDevice(c fiber.Ctx) error {
	log.Println("registering new fleet device")
	return c.SendString("registered new fleet device")
}
