package warehouse

import (
	"context"
	"errors"
	"strings"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var ErrInvalidInput = errors.New("invalid warehouse input")
var ErrNotFound = errors.New("warehouse not found")
var ErrForbidden = errors.New("warehouse permission denied")

type Warehouse struct {
	ID        int64  `json:"id"`
	CompanyID int64  `json:"company_id"`
	TypeID    int64  `json:"type_id"`
	TypeName  string `json:"type_name"`
	Name      string `json:"name"`
	Address   string `json:"address,omitempty"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

type Input struct{ Name, TypeName, Address string }

type Repository interface {
	List(ctx context.Context, companyID int64) ([]Warehouse, error)
	Create(ctx context.Context, companyID int64, input Input) (Warehouse, error)
	Get(ctx context.Context, companyID, id int64) (Warehouse, error)
	Update(ctx context.Context, companyID, id int64, input Input) (Warehouse, error)
	Delete(ctx context.Context, companyID, id int64) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func normalize(input Input) Input {
	input.Name = strings.TrimSpace(input.Name)
	input.TypeName = strings.TrimSpace(input.TypeName)
	input.Address = strings.TrimSpace(input.Address)
	return input
}

func (s *Service) List(ctx context.Context, actor rbac.Actor) ([]Warehouse, error) {
	if !actor.Can(constants.PermissionWarehouseView, nil) {
		return nil, ErrForbidden
	}
	return s.repository.List(ctx, actor.CompanyID)
}
func (s *Service) Create(ctx context.Context, actor rbac.Actor, input Input) (Warehouse, error) {
	if !actor.Can(constants.PermissionWarehousesManage, nil) {
		return Warehouse{}, ErrForbidden
	}
	input = normalize(input)
	if input.Name == "" || input.TypeName == "" {
		return Warehouse{}, ErrInvalidInput
	}
	return s.repository.Create(ctx, actor.CompanyID, input)
}
func (s *Service) Get(ctx context.Context, actor rbac.Actor, id int64) (Warehouse, error) {
	if !actor.Can(constants.PermissionWarehouseView, nil) {
		return Warehouse{}, ErrForbidden
	}
	return s.repository.Get(ctx, actor.CompanyID, id)
}
func (s *Service) Update(ctx context.Context, actor rbac.Actor, id int64, input Input) (Warehouse, error) {
	if !actor.Can(constants.PermissionWarehousesManage, nil) {
		return Warehouse{}, ErrForbidden
	}
	input = normalize(input)
	if input.Name == "" || input.TypeName == "" {
		return Warehouse{}, ErrInvalidInput
	}
	return s.repository.Update(ctx, actor.CompanyID, id, input)
}
func (s *Service) Delete(ctx context.Context, actor rbac.Actor, id int64) error {
	if !actor.Can(constants.PermissionWarehousesManage, nil) {
		return ErrForbidden
	}
	return s.repository.Delete(ctx, actor.CompanyID, id)
}
