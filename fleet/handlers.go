package fleet

import (
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
	fleet.Post("/register", handler.RegisterFleetDevice)
	fleet.Delete("/:uid", handler.UnregisterFleetDevice)
}

// RegisterFleetDevice connects a new device to the network
func (h *FleetHandler) RegisterFleetDevice(c fiber.Ctx) error {
	req := new(RegisterDeviceReq)
	if err := c.Bind().JSON(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	id, err := h.service.RegisterDevice(*req)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{
		"message": "device registered successfully",
		"uuid":    id,
	})
}

func (h *FleetHandler) UnregisterFleetDevice(c fiber.Ctx) error {
	uid := c.Params("uid")
}
