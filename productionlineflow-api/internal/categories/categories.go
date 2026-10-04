package categories

import (
	"context"
	"errors"
	"strings"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var (
	ErrForbidden    = errors.New("category permission denied")
	ErrInvalidInput = errors.New("invalid category input")
	ErrNotFound     = errors.New("category not found")
	ErrConflict     = errors.New("category is in use or already exists")
)

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Repository interface {
	List(context.Context, int64) ([]Category, error)
	Create(context.Context, int64, string) (Category, error)
	Update(context.Context, int64, int64, string) (Category, error)
	Delete(context.Context, int64, int64) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, actor rbac.Actor) ([]Category, error) {
	if !actor.Can(constants.PermissionItemCategoriesView, nil) && !actor.Can(constants.PermissionItemCategoriesManage, nil) {
		return nil, ErrForbidden
	}
	return s.repository.List(ctx, actor.CompanyID)
}

func (s *Service) Create(ctx context.Context, actor rbac.Actor, name string) (Category, error) {
	if !actor.Can(constants.PermissionItemCategoriesManage, nil) {
		return Category{}, ErrForbidden
	}
	name, err := normalizeName(name)
	if err != nil {
		return Category{}, err
	}
	return s.repository.Create(ctx, actor.CompanyID, name)
}

func (s *Service) Update(ctx context.Context, actor rbac.Actor, id int64, name string) (Category, error) {
	if !actor.Can(constants.PermissionItemCategoriesManage, nil) {
		return Category{}, ErrForbidden
	}
	if id < 1 {
		return Category{}, ErrInvalidInput
	}
	name, err := normalizeName(name)
	if err != nil {
		return Category{}, err
	}
	return s.repository.Update(ctx, actor.CompanyID, id, name)
}

func (s *Service) Delete(ctx context.Context, actor rbac.Actor, id int64) error {
	if !actor.Can(constants.PermissionItemCategoriesManage, nil) {
		return ErrForbidden
	}
	if id < 1 {
		return ErrInvalidInput
	}
	return s.repository.Delete(ctx, actor.CompanyID, id)
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return "", ErrInvalidInput
	}
	return name, nil
}
