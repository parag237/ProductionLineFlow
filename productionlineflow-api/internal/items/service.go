package items

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var (
	ErrForbidden       = errors.New("item permission denied")
	ErrInvalidInput    = errors.New("invalid item input")
	ErrNotFound        = errors.New("item or production run not found")
	ErrConflict        = errors.New("item or production run conflict")
	ErrInvalidProgress = errors.New("invalid production progress")
)

type Step struct {
	ID           int64  `json:"id"`
	Position     int    `json:"position"`
	Title        string `json:"title"`
	Instructions string `json:"instructions,omitempty"`
}

type Item struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	SKU         string `json:"sku,omitempty"`
	Description string `json:"description,omitempty"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Steps       []Step `json:"steps"`
}

type Input struct {
	Name        string      `json:"name" binding:"required"`
	SKU         string      `json:"sku"`
	Description string      `json:"description"`
	Steps       []StepInput `json:"steps" binding:"required,min=1,dive"`
}

type StepInput struct {
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
}

type Repository interface {
	ListItems(context.Context, int64) ([]Item, error)
	GetItem(context.Context, int64, int64) (Item, error)
	CreateItem(context.Context, int64, Input) (Item, error)
	UpdateItem(context.Context, int64, int64, Input) (Item, error)
	ArchiveItem(context.Context, int64, int64) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, actor rbac.Actor) ([]Item, error) {
	if !actor.Can(constants.PermissionItemsView, nil) && !actor.Can(constants.PermissionItemsManage, nil) && !actor.Can(constants.PermissionProductionExecute, nil) {
		return nil, ErrForbidden
	}
	return s.repository.ListItems(ctx, actor.CompanyID)
}

func (s *Service) Get(ctx context.Context, actor rbac.Actor, id int64) (Item, error) {
	if !actor.Can(constants.PermissionItemsView, nil) && !actor.Can(constants.PermissionItemsManage, nil) && !actor.Can(constants.PermissionProductionExecute, nil) {
		return Item{}, ErrForbidden
	}
	return s.repository.GetItem(ctx, actor.CompanyID, id)
}

func (s *Service) Create(ctx context.Context, actor rbac.Actor, input Input) (Item, error) {
	if !actor.Can(constants.PermissionItemsManage, nil) {
		return Item{}, ErrForbidden
	}
	input, err := normalizeInput(input)
	if err != nil {
		return Item{}, err
	}
	return s.repository.CreateItem(ctx, actor.CompanyID, input)
}

func (s *Service) Update(ctx context.Context, actor rbac.Actor, id int64, input Input) (Item, error) {
	if !actor.Can(constants.PermissionItemsManage, nil) {
		return Item{}, ErrForbidden
	}
	input, err := normalizeInput(input)
	if err != nil {
		return Item{}, err
	}
	return s.repository.UpdateItem(ctx, actor.CompanyID, id, input)
}

func (s *Service) Archive(ctx context.Context, actor rbac.Actor, id int64) error {
	if !actor.Can(constants.PermissionItemsManage, nil) {
		return ErrForbidden
	}
	return s.repository.ArchiveItem(ctx, actor.CompanyID, id)
}

func normalizeInput(input Input) (Input, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.SKU = strings.TrimSpace(input.SKU)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || len(input.Name) > 120 || len(input.SKU) > 64 || len(input.Description) > 2000 || len(input.Steps) == 0 {
		return Input{}, ErrInvalidInput
	}
	for index := range input.Steps {
		input.Steps[index].Title = strings.TrimSpace(input.Steps[index].Title)
		input.Steps[index].Instructions = strings.TrimSpace(input.Steps[index].Instructions)
		if input.Steps[index].Title == "" || len(input.Steps[index].Title) > 120 || len(input.Steps[index].Instructions) > 2000 {
			return Input{}, fmt.Errorf("%w: invalid step %d", ErrInvalidInput, index+1)
		}
	}
	return input, nil
}
