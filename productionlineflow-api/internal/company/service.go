package company

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var ErrInvalidInput = errors.New("invalid company onboarding input")
var ErrDuplicateSlug = errors.New("company slug already exists")
var ErrDuplicateAdminEmail = errors.New("initial super admin email already exists")
var ErrForbidden = errors.New("platform actor cannot create companies")
var ErrNotFound = errors.New("company not found")

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Input struct {
	Name               string
	Slug               string
	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPassword string
}

type Result struct {
	ID              int64
	Slug            string
	Name            string
	SuperAdminID    int64
	SuperAdminName  string
	SuperAdminEmail string
}

type Company struct {
	ID          int64      `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	SuspendedAt *time.Time `json:"suspended_at,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
}

type UpdateInput struct {
	Name string
	Slug string
}

type Repository interface {
	CreateCompany(ctx context.Context, input Input, passwordHash string) (Result, error)
	ListCompanies(ctx context.Context) ([]Company, error)
	UpdateCompany(ctx context.Context, id int64, input UpdateInput) (Company, error)
	SuspendCompany(ctx context.Context, id int64) (Company, error)
	ReactivateCompany(ctx context.Context, id int64) (Company, error)
}

func (s *Service) List(ctx context.Context, actor rbac.PlatformActor) ([]Company, error) {
	if !actor.Can(constants.PermissionCompaniesManage) {
		return nil, ErrForbidden
	}
	return s.repository.ListCompanies(ctx)
}

func (s *Service) Update(ctx context.Context, actor rbac.PlatformActor, id int64, input UpdateInput) (Company, error) {
	if !actor.Can(constants.PermissionCompaniesManage) {
		return Company{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	if input.Name == "" || !slugPattern.MatchString(input.Slug) {
		return Company{}, ErrInvalidInput
	}
	return s.repository.UpdateCompany(ctx, id, input)
}

func (s *Service) Suspend(ctx context.Context, actor rbac.PlatformActor, id int64) (Company, error) {
	if !actor.Can(constants.PermissionCompaniesManage) {
		return Company{}, ErrForbidden
	}
	return s.repository.SuspendCompany(ctx, id)
}

func (s *Service) Reactivate(ctx context.Context, actor rbac.PlatformActor, id int64) (Company, error) {
	if !actor.Can(constants.PermissionCompaniesManage) {
		return Company{}, ErrForbidden
	}
	return s.repository.ReactivateCompany(ctx, id)
}

type Service struct {
	repository  Repository
	minPassword int
}

func NewService(repository Repository, minPassword int) *Service {
	return &Service{repository: repository, minPassword: minPassword}
}

func (s *Service) Create(ctx context.Context, actor rbac.PlatformActor, input Input) (Result, error) {
	if !actor.Can(constants.PermissionCompaniesCreate) {
		return Result{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Slug = strings.TrimSpace(strings.ToLower(input.Slug))
	input.SuperAdminName = strings.TrimSpace(input.SuperAdminName)
	input.SuperAdminEmail = strings.TrimSpace(input.SuperAdminEmail)
	if input.Name == "" || !slugPattern.MatchString(input.Slug) || input.SuperAdminName == "" || input.SuperAdminEmail == "" || len(input.SuperAdminPassword) < s.minPassword {
		return Result{}, ErrInvalidInput
	}
	passwordHash, err := auth.HashPassword(input.SuperAdminPassword)
	if err != nil {
		return Result{}, fmt.Errorf("hash super admin password: %w", err)
	}
	return s.repository.CreateCompany(ctx, input, passwordHash)
}
