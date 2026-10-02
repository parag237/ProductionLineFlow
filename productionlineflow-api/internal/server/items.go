package server

import (
	"errors"
	"net/http"
	"strconv"

	itemsModule "productionlineflow-api/internal/items"

	"github.com/gin-gonic/gin"
)

type ItemHandler struct {
	catalog    *itemsModule.Service
	production *itemsModule.ProductionService
}

func NewItemHandler(catalog *itemsModule.Service, production *itemsModule.ProductionService) *ItemHandler {
	return &ItemHandler{catalog: catalog, production: production}
}

func (h *ItemHandler) List(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	items, err := h.catalog.List(c, actor)
	if err != nil {
		h.writeItemError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ItemHandler) Get(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid item id")
		return
	}
	item, err := h.catalog.Get(c, actor, id)
	if err != nil {
		h.writeItemError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *ItemHandler) Create(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var input itemsModule.Input
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid item request")
		return
	}
	item, err := h.catalog.Create(c, actor, input)
	if err != nil {
		h.writeItemError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *ItemHandler) Update(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid item id")
		return
	}
	var input itemsModule.Input
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid item request")
		return
	}
	item, err := h.catalog.Update(c, actor, id, input)
	if err != nil {
		h.writeItemError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *ItemHandler) Archive(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid item id")
		return
	}
	if err := h.catalog.Archive(c, actor, id); err != nil {
		h.writeItemError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ItemHandler) ListRuns(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	runs, err := h.production.List(c, actor)
	if err != nil {
		h.writeProductionError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func (h *ItemHandler) GetRun(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("runID"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid production run id")
		return
	}
	run, err := h.production.Get(c, actor, id)
	if err != nil {
		h.writeProductionError(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *ItemHandler) CreateRun(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var input itemsModule.RunInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid production run request")
		return
	}
	run, err := h.production.Create(c, actor, input)
	if err != nil {
		h.writeProductionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, run)
}

func (h *ItemHandler) CompleteStep(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	runID, err := strconv.ParseInt(c.Param("runID"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid production run id")
		return
	}
	stepID, err := strconv.ParseInt(c.Param("stepID"), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid production step id")
		return
	}
	var input itemsModule.StepCompletionInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid step completion request")
		return
	}
	run, err := h.production.CompleteStep(c, actor, runID, stepID, input)
	if err != nil {
		h.writeProductionError(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *ItemHandler) writeItemError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, itemsModule.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Item permission required")
	case errors.Is(err, itemsModule.ErrInvalidInput):
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid item request")
	case errors.Is(err, itemsModule.ErrNotFound):
		writeError(c, http.StatusNotFound, "item_not_found", "Item not found")
	case errors.Is(err, itemsModule.ErrConflict):
		writeError(c, http.StatusConflict, "item_conflict", "An item with this SKU already exists")
	default:
		logInternalError(c, "manage items", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to manage items")
	}
}

func (h *ItemHandler) writeProductionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, itemsModule.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Production permission required")
	case errors.Is(err, itemsModule.ErrInvalidTracking):
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid production request")
	case errors.Is(err, itemsModule.ErrInvalidProgress):
		writeError(c, http.StatusConflict, "invalid_progress", "Production step is out of order or already complete")
	case errors.Is(err, itemsModule.ErrNotFound):
		writeError(c, http.StatusNotFound, "production_not_found", "Item or production run not found")
	case errors.Is(err, itemsModule.ErrConflict):
		writeError(c, http.StatusConflict, "production_conflict", "Production tracking data conflicts with an existing record")
	default:
		logInternalError(c, "manage production", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to manage production")
	}
}
