package server

import (
	"errors"
	"net/http"
	"strconv"

	categoryModule "productionlineflow-api/internal/categories"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct{ service *categoryModule.Service }

func NewCategoryHandler(service *categoryModule.Service) *CategoryHandler {
	return &CategoryHandler{service: service}
}

type categoryRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *CategoryHandler) List(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	categories, err := h.service.List(c.Request.Context(), actor)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *CategoryHandler) Create(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	var input categoryRequest
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid category request")
		return
	}
	category, err := h.service.Create(c.Request.Context(), actor, input.Name)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, category)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid category id")
		return
	}
	var input categoryRequest
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid category request")
		return
	}
	category, err := h.service.Update(c.Request.Context(), actor, id, input.Name)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, category)
}

func (h *CategoryHandler) Delete(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(c, http.StatusBadRequest, "invalid_id", "Invalid category id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), actor, id); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CategoryHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, categoryModule.ErrForbidden):
		writeError(c, http.StatusForbidden, "forbidden", "Category permission required")
	case errors.Is(err, categoryModule.ErrInvalidInput):
		writeError(c, http.StatusBadRequest, "invalid_request", "Invalid category request")
	case errors.Is(err, categoryModule.ErrNotFound):
		writeError(c, http.StatusNotFound, "category_not_found", "Category not found")
	case errors.Is(err, categoryModule.ErrConflict):
		writeError(c, http.StatusConflict, "category_in_use", "Category name already exists or category is assigned to items")
	default:
		logInternalError(c, "manage item categories", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "Unable to manage item categories")
	}
}
