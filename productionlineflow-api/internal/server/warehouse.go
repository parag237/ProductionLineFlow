package server

import (
	"errors"
	"net/http"
	"strconv"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
	"productionlineflow-api/internal/warehouse"

	"github.com/gin-gonic/gin"
)

type WarehouseHandler struct{ service *warehouse.Service }

func NewWarehouseHandler(service *warehouse.Service) *WarehouseHandler {
	return &WarehouseHandler{service: service}
}

type warehouseRequest struct {
	Name     string `json:"name" binding:"required"`
	TypeName string `json:"type_name" binding:"required"`
	Address  string `json:"address"`
}

func tenantActor(c *gin.Context) (rbac.Actor, bool) {
	value, ok := c.Get(constants.ActorContextKey)
	actor, valid := value.(rbac.Actor)
	return actor, ok && valid
}
func (h *WarehouseHandler) actor(c *gin.Context) (rbac.Actor, bool) { return tenantActor(c) }

func (h *WarehouseHandler) List(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	items, err := h.service.List(c, actor)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, gin.H{"warehouses": items})
}
func (h *WarehouseHandler) Create(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	var req warehouseRequest
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, 400, "invalid_request", "Invalid warehouse request")
		return
	}
	item, err := h.service.Create(c, actor, warehouse.Input{Name: req.Name, TypeName: req.TypeName, Address: req.Address})
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (h *WarehouseHandler) Get(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid warehouse id")
		return
	}
	item, err := h.service.Get(c, actor, id)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, item)
}
func (h *WarehouseHandler) Update(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid warehouse id")
		return
	}
	var req warehouseRequest
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, 400, "invalid_request", "Invalid warehouse request")
		return
	}
	item, err := h.service.Update(c, actor, id, warehouse.Input{Name: req.Name, TypeName: req.TypeName, Address: req.Address})
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, item)
}
func (h *WarehouseHandler) Delete(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid warehouse id")
		return
	}
	if err := h.service.Delete(c, actor, id); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *WarehouseHandler) error(c *gin.Context, err error) {
	switch {
	case errors.Is(err, warehouse.ErrForbidden):
		writeError(c, 403, "forbidden", "Warehouse permission required")
	case errors.Is(err, warehouse.ErrInvalidInput):
		writeError(c, 400, "invalid_request", "Invalid warehouse request")
	case errors.Is(err, warehouse.ErrNotFound):
		writeError(c, 404, "warehouse_not_found", "Warehouse not found")
	case errors.Is(err, warehouse.ErrConflict):
		writeError(c, 409, "warehouse_in_use", "Warehouse is referenced by existing records and cannot be removed")
	default:
		logInternalError(c, "manage warehouse", err)
		writeError(c, 500, "internal_error", "Unable to manage warehouse")
	}
}
