package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"productionlineflow-api/internal/people"
)

type PeopleHandler struct{ service *people.Service }

func NewPeopleHandler(service *people.Service) *PeopleHandler {
	return &PeopleHandler{service: service}
}

func (h *PeopleHandler) List(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	status := c.DefaultQuery("status", "active")
	if status != "active" && status != "inactive" && status != "all" {
		writeError(c, 400, "invalid_request", "Invalid status filter")
		return
	}
	items, err := h.service.ListPeople(c, actor, c.Query("q"), status != "active")
	if err != nil {
		h.error(c, err)
		return
	}
	if status == "inactive" {
		inactive := make([]people.Person, 0)
		for _, item := range items {
			if !item.IsActive {
				inactive = append(inactive, item)
			}
		}
		items = inactive
	}
	c.JSON(200, gin.H{"people": items})
}

func (h *PeopleHandler) Get(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	item, err := h.service.GetPerson(c, actor, id)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, item)
}

func (h *PeopleHandler) Create(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	var input people.CreateInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid person request")
		return
	}
	item, err := h.service.CreatePerson(c, actor, input)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *PeopleHandler) Update(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	var input people.UpdateInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid person request")
		return
	}
	item, err := h.service.UpdatePerson(c, actor, id, input)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, item)
}

func (h *PeopleHandler) Deactivate(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	if err := h.service.DeactivatePerson(c, actor, id); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PeopleHandler) ResetPassword(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid password request")
		return
	}
	if err := h.service.ResetPassword(c, actor, id, input.Password); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PeopleHandler) ChangeOwnPassword(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid password request")
		return
	}
	if err := h.service.ChangeOwnPassword(c, actor, input.CurrentPassword, input.NewPassword); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PeopleHandler) AddAssignment(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	var input people.AssignmentInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid role assignment")
		return
	}
	item, err := h.service.AddAssignment(c, actor, id, input)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *PeopleHandler) RemoveAssignment(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	personID, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	assignmentID, err := strconv.ParseInt(c.Param("assignmentId"), 10, 64)
	if err != nil || assignmentID <= 0 {
		writeError(c, 400, "invalid_id", "Invalid assignment id")
		return
	}
	if err := h.service.RemoveAssignment(c, actor, personID, assignmentID); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PeopleHandler) TransferSuperAdmin(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	targetID, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid person id")
		return
	}
	if err := h.service.TransferSuperAdmin(c, actor, targetID); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PeopleHandler) ListRoles(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	items, err := h.service.ListRoles(c, actor)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, gin.H{"roles": items})
}

func (h *PeopleHandler) ListPermissionCatalog(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	items, err := h.service.ListPermissionCatalog(c, actor)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, gin.H{"permissions": items})
}

func (h *PeopleHandler) CreateRole(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	var input people.RoleInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid role request")
		return
	}
	item, err := h.service.CreateRole(c, actor, input)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *PeopleHandler) UpdateRole(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid role id")
		return
	}
	var input people.RoleUpdateInput
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "invalid_request", "Invalid role request")
		return
	}
	item, err := h.service.UpdateRole(c, actor, id, input)
	if err != nil {
		h.error(c, err)
		return
	}
	c.JSON(200, item)
}

func (h *PeopleHandler) DeleteRole(c *gin.Context) {
	actor, ok := tenantActor(c)
	if !ok {
		writeError(c, 401, "unauthorized", "Authentication required")
		return
	}
	id, err := parseID(c)
	if err != nil {
		writeError(c, 400, "invalid_id", "Invalid role id")
		return
	}
	if err := h.service.DeleteRole(c, actor, id); err != nil {
		h.error(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func (h *PeopleHandler) error(c *gin.Context, err error) {
	switch {
	case errors.Is(err, people.ErrForbidden):
		writeError(c, 403, "forbidden", "Permission required")
	case errors.Is(err, people.ErrNotFound):
		writeError(c, 404, "not_found", "Person or role not found")
	case errors.Is(err, people.ErrInvalidInput):
		writeError(c, 400, "invalid_request", "Invalid people or role data")
	case errors.Is(err, people.ErrInvalidPassword):
		writeError(c, 401, "invalid_password", "Current password is incorrect")
	case errors.Is(err, people.ErrConflict), errors.Is(err, people.ErrLastSuperAdmin):
		writeError(c, 409, "conflict", err.Error())
	default:
		writeError(c, 500, "internal_error", "Unable to manage people or roles")
	}
}
