package fleet

import (
	"log"

	"github.com/gofiber/fiber/v3"
)

// FleetHandler TODO: add stateful service/dbrepo
type FleetHandler struct {
	service *DeviceService
}

func NewFleetHandler(s *DeviceService) *FleetHandler {
	return &FleetHandler{
		service: s,
	}
}

func RegisterFleetRoutes(router fiber.Router, handler *FleetHandler) {
	fleet := router.Group("/fleet")
	fleet.Get("/register", handler.RegisterFleetDevice)
}

// RegisterFleetDevice connects a new device to the network
func (h *FleetHandler) RegisterFleetDevice(c fiber.Ctx) error {
	// TODO: register a generic fleet device and have irrigation settings be join table?
	log.Println("registering new fleet device")
	err := h.service.RegisterDevice(RegisterDeviceReq{})
	if err != nil {
		return c.Err()
	}
	return c.SendString("registered new fleet device")
}
