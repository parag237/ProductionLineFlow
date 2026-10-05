package server

import (
	"errors"
	"net/http"

	itemsModule "productionlineflow-api/internal/items"

	"github.com/gin-gonic/gin"
)

type ItemHandler struct {
	catalog *itemsModule.Service
}

func NewItemHandler(catalog *itemsModule.Service) *ItemHandler {
	return &ItemHandler{catalog: catalog}
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
