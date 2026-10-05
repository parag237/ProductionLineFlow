package server

import (
	"errors"
	"net/http"
	"strconv"

	operationsModule "productionlineflow-api/internal/operations"

	"github.com/gin-gonic/gin"
)

type OperationLogHandler struct{ service *operationsModule.Service }

func NewOperationLogHandler(service *operationsModule.Service) *OperationLogHandler {
	return &OperationLogHandler{service: service}
}

func (h *OperationLogHandler) Options(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var warehouseID *int64
	if value := c.Query("warehouse_id"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			writeError(c, http.StatusBadRequest, "invalid_id", "Invalid warehouse id")
			return
		}
		warehouseID = &parsed
	}
	options, err := h.service.Options(c.Request.Context(), actor, warehouseID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, options)
}

func (h *OperationLogHandler) List(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var warehouseID *int64
	if value := c.Query("warehouse_id"); value != "" && value != "all" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			writeError(c, http.StatusBadRequest, "invalid_id", "Invalid warehouse id")
			return
		}
		warehouseID = &parsed
	}
	entries, err := h.service.List(c.Request.Context(), actor, warehouseID, c.Query("work_date"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

func (h *OperationLogHandler) Analyze(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var warehouseID *int64
	if value := c.Query("warehouse_id"); value != "" && value != "all" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 1 {
			writeError(c, http.StatusBadRequest, "invalid_id", "Invalid warehouse id")
			return
		}
		warehouseID = &parsed
	}
	analysis, err := h.service.Analyze(c.Request.Context(), actor, warehouseID, c.Query("from_date"), c.Query("to_date"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, analysis)
}

func (h *OperationLogHandler) Create(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var input operationsModule.Input
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid operation log request")
		return
	}
	entry, err := h.service.Create(c.Request.Context(), actor, input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h *OperationLogHandler) Update(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid operation log id")
		return
	}
	var input operationsModule.Input
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid operation log request")
		return
	}
	entry, err := h.service.Update(c.Request.Context(), actor, id, input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (h *OperationLogHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, operationsModule.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Operation log permission required")
	case errors.Is(err, operationsModule.ErrDateOverrideForbidden):
		writeError(c, http.StatusForbidden, "date_override_forbidden", "Only administrators can choose a work date other than today")
	case errors.Is(err, operationsModule.ErrInvalidInput):
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid operation log request")
	case errors.Is(err, operationsModule.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "Operation log not found")
	default:
		logInternalError(c, "manage operation logs", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to manage operation logs")
	}
}
