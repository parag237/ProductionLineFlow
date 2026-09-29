package company

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var ErrInvalidInput = errors.New("invalid company onboarding input")
var ErrDuplicateSlug = errors.New("company slug already exists")
var ErrDuplicateAdminEmail = errors.New("initial super admin email already exists")
var ErrForbidden = errors.New("platform actor cannot create companies")

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

type Repository interface {
	CreateCompany(ctx context.Context, input Input, passwordHash string) (Result, error)
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
