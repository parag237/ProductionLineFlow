package people

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var (
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("person or role not found")
	ErrInvalidInput    = errors.New("invalid people or role input")
	ErrConflict        = errors.New("people or role conflict")
	ErrLastSuperAdmin  = errors.New("the active Super Admin cannot be removed or deactivated")
	ErrInvalidPassword = errors.New("current password is invalid")
)

type Assignment struct {
	ID            int64  `json:"id"`
	RoleID        int64  `json:"role_id"`
	RoleSlug      string `json:"role_slug"`
	RoleName      string `json:"role_name"`
	RoleScope     string `json:"role_scope"`
	WarehouseID   *int64 `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name,omitempty"`
}

type Person struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Email       string       `json:"email"`
	IsActive    bool         `json:"is_active"`
	CreatedAt   string       `json:"created_at"`
	Assignments []Assignment `json:"assignments"`
}

type Role struct {
	ID          int64    `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions"`
}
type Permission struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type AssignmentInput struct {
	RoleID      int64  `json:"role_id"`
	WarehouseID *int64 `json:"warehouse_id"`
}

type CreateInput struct {
	Name        string            `json:"name"`
	Email       string            `json:"email"`
	Password    string            `json:"password"`
	Assignments []AssignmentInput `json:"assignments"`
}

type UpdateInput struct {
	Name     *string `json:"name"`
	Email    *string `json:"email"`
	IsActive *bool   `json:"is_active"`
}

type RoleInput struct {
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

type RoleUpdateInput struct {
	Name        *string  `json:"name"`
	Permissions []string `json:"permissions"`
}

type AssignmentPolicy struct {
	SuperAdmin  bool
	CompanyUser bool
	Warehouses  []int64
}

type Repository interface {
	ListPeople(context.Context, int64, *int64, []int64, bool, bool) ([]Person, error)
	GetPerson(context.Context, int64, int64, []int64, bool) (Person, error)
	CreatePerson(context.Context, int64, int64, CreateInput, string, AssignmentPolicy) (Person, error)
	UpdatePerson(context.Context, int64, int64, UpdateInput, bool) (Person, error)
	SetPassword(context.Context, int64, int64, string, bool) error
	PasswordHash(context.Context, int64, int64) (string, error)
	AddAssignment(context.Context, int64, int64, int64, AssignmentInput, AssignmentPolicy) (Assignment, error)
	RemoveAssignment(context.Context, int64, int64, int64, int64, AssignmentPolicy) error
	TransferSuperAdmin(context.Context, int64, int64, int64) error
	ListRoles(context.Context, int64) ([]Role, error)
	ListPermissionCatalog(context.Context) ([]Permission, error)
	CreateRole(context.Context, int64, RoleInput, string) (Role, error)
	UpdateRole(context.Context, int64, int64, RoleUpdateInput) (Role, error)
	DeleteRole(context.Context, int64, int64) error
}

type Service struct {
	repo        Repository
	minPassword int
}

func NewService(repo Repository, minPassword int) *Service {
	return &Service{repo: repo, minPassword: minPassword}
}

func managedWarehouses(actor rbac.Actor) []int64 {
	seen := map[int64]bool{}
	var ids []int64
	for _, assignment := range actor.Assignments {
		if assignment.Permission == constants.PermissionWarehouseMembersManage && assignment.WarehouseID != nil && !seen[*assignment.WarehouseID] {
			seen[*assignment.WarehouseID] = true
			ids = append(ids, *assignment.WarehouseID)
		}
	}
	return ids
}

func assignmentPolicy(actor rbac.Actor) AssignmentPolicy {
	return AssignmentPolicy{
		SuperAdmin:  actor.Can(constants.PermissionAdminsManage, nil),
		CompanyUser: actor.Can(constants.PermissionUsersCreate, nil),
		Warehouses:  managedWarehouses(actor),
	}
}

func (s *Service) ListPeople(ctx context.Context, actor rbac.Actor, query string, includeInactive bool) ([]Person, error) {
	if actor.CompanyID == 0 {
		return nil, ErrForbidden
	}
	if actor.Can(constants.PermissionUsersCreate, nil) {
		people, err := s.repo.ListPeople(ctx, actor.CompanyID, nil, nil, false, includeInactive)
		return filterPeople(people, query), err
	}
	ids := managedWarehouses(actor)
	if len(ids) == 0 {
		return nil, ErrForbidden
	}
	people, err := s.repo.ListPeople(ctx, actor.CompanyID, nil, ids, true, false)
	if err != nil {
		return people, err
	}
	return filterPeople(people, query), nil
}

func filterPeople(people []Person, query string) []Person {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return people
	}
	filtered := make([]Person, 0, len(people))
	for _, person := range people {
		if strings.Contains(strings.ToLower(person.Name), query) || strings.Contains(strings.ToLower(person.Email), query) {
			filtered = append(filtered, person)
		}
	}
	return filtered
}

func (s *Service) GetPerson(ctx context.Context, actor rbac.Actor, id int64) (Person, error) {
	if actor.Can(constants.PermissionUsersCreate, nil) {
		return s.repo.GetPerson(ctx, actor.CompanyID, id, nil, false)
	}
	ids := managedWarehouses(actor)
	if len(ids) == 0 {
		return Person{}, ErrForbidden
	}
	person, err := s.repo.GetPerson(ctx, actor.CompanyID, id, ids, true)
	if err != nil {
		return Person{}, err
	}
	if !person.IsActive {
		return Person{}, ErrNotFound
	}
	return person, nil
}

func validEmail(value string) bool {
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	return err == nil && parsed.Address == strings.TrimSpace(value) && strings.Contains(parsed.Address, "@")
}

func (s *Service) CreatePerson(ctx context.Context, actor rbac.Actor, input CreateInput) (Person, error) {
	policy := assignmentPolicy(actor)
	if !policy.CompanyUser && len(policy.Warehouses) == 0 {
		return Person{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.TrimSpace(input.Email)
	if input.Name == "" || !validEmail(input.Email) || len(input.Password) < s.minPassword {
		return Person{}, ErrInvalidInput
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return Person{}, fmt.Errorf("hash person password: %w", err)
	}
	return s.repo.CreatePerson(ctx, actor.CompanyID, actor.UserID, input, passwordHash, policy)
}

func (s *Service) UpdatePerson(ctx context.Context, actor rbac.Actor, id int64, input UpdateInput) (Person, error) {
	if !actor.Can(constants.PermissionUsersCreate, nil) {
		return Person{}, ErrForbidden
	}
	if input.Name != nil {
		*input.Name = strings.TrimSpace(*input.Name)
		if *input.Name == "" {
			return Person{}, ErrInvalidInput
		}
	}
	if input.Email != nil {
		*input.Email = strings.TrimSpace(*input.Email)
		if !validEmail(*input.Email) {
			return Person{}, ErrInvalidInput
		}
	}
	return s.repo.UpdatePerson(ctx, actor.CompanyID, id, input, actor.Can(constants.PermissionAdminsManage, nil))
}

func (s *Service) DeactivatePerson(ctx context.Context, actor rbac.Actor, id int64) error {
	if !actor.Can(constants.PermissionUsersCreate, nil) {
		return ErrForbidden
	}
	active := false
	_, err := s.repo.UpdatePerson(ctx, actor.CompanyID, id, UpdateInput{IsActive: &active}, actor.Can(constants.PermissionAdminsManage, nil))
	return err
}

func (s *Service) ResetPassword(ctx context.Context, actor rbac.Actor, id int64, password string) error {
	if !actor.Can(constants.PermissionUsersCreate, nil) {
		return ErrForbidden
	}
	if len(password) < s.minPassword {
		return ErrInvalidInput
	}
	if _, err := s.repo.GetPerson(ctx, actor.CompanyID, id, nil, false); err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.SetPassword(ctx, actor.CompanyID, id, hash, actor.Can(constants.PermissionAdminsManage, nil))
}

func (s *Service) ChangeOwnPassword(ctx context.Context, actor rbac.Actor, currentPassword, newPassword string) error {
	if len(newPassword) < s.minPassword {
		return ErrInvalidInput
	}
	hash, err := s.repo.PasswordHash(ctx, actor.CompanyID, actor.UserID)
	if err != nil {
		return err
	}
	if !auth.VerifyPassword(currentPassword, hash) {
		return ErrInvalidPassword
	}
	newHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.SetPassword(ctx, actor.CompanyID, actor.UserID, newHash, true)
}

func (s *Service) AddAssignment(ctx context.Context, actor rbac.Actor, userID int64, input AssignmentInput) (Assignment, error) {
	policy := assignmentPolicy(actor)
	if !policy.SuperAdmin && !policy.CompanyUser && len(policy.Warehouses) == 0 {
		return Assignment{}, ErrForbidden
	}
	return s.repo.AddAssignment(ctx, actor.CompanyID, actor.UserID, userID, input, policy)
}

func (s *Service) RemoveAssignment(ctx context.Context, actor rbac.Actor, userID, assignmentID int64) error {
	policy := assignmentPolicy(actor)
	if !policy.SuperAdmin && !policy.CompanyUser && len(policy.Warehouses) == 0 {
		return ErrForbidden
	}
	return s.repo.RemoveAssignment(ctx, actor.CompanyID, actor.UserID, userID, assignmentID, policy)
}

func (s *Service) TransferSuperAdmin(ctx context.Context, actor rbac.Actor, targetID int64) error {
	if !actor.Can(constants.PermissionAdminsManage, nil) || targetID == actor.UserID {
		return ErrForbidden
	}
	return s.repo.TransferSuperAdmin(ctx, actor.CompanyID, actor.UserID, targetID)
}

func (s *Service) ListRoles(ctx context.Context, actor rbac.Actor) ([]Role, error) {
	if actor.CompanyID == 0 || (!actor.Can(constants.PermissionUsersCreate, nil) && len(managedWarehouses(actor)) == 0) {
		return nil, ErrForbidden
	}
	return s.repo.ListRoles(ctx, actor.CompanyID)
}

func (s *Service) ListPermissionCatalog(ctx context.Context, actor rbac.Actor) ([]Permission, error) {
	if !actor.Can(constants.PermissionRolesManage, nil) {
		return nil, ErrForbidden
	}
	return s.repo.ListPermissionCatalog(ctx)
}

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func roleSlug(name string) string {
	return strings.Trim(slugSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
}

func validRoleInput(name, scope string, permissions []string) bool {
	if strings.TrimSpace(name) == "" || (scope != "company" && scope != "warehouse") || len(permissions) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, permission := range permissions {
		if strings.TrimSpace(permission) == "" || seen[permission] || permission == constants.PermissionAdminsManage || permission == constants.PermissionRolesManage {
			return false
		}
		seen[permission] = true
	}
	return true
}

func (s *Service) CreateRole(ctx context.Context, actor rbac.Actor, input RoleInput) (Role, error) {
	if !actor.Can(constants.PermissionRolesManage, nil) {
		return Role{}, ErrForbidden
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validRoleInput(input.Name, input.Scope, input.Permissions) || roleSlug(input.Name) == "" {
		return Role{}, ErrInvalidInput
	}
	return s.repo.CreateRole(ctx, actor.CompanyID, input, roleSlug(input.Name))
}

func (s *Service) UpdateRole(ctx context.Context, actor rbac.Actor, id int64, input RoleUpdateInput) (Role, error) {
	if !actor.Can(constants.PermissionRolesManage, nil) {
		return Role{}, ErrForbidden
	}
	if input.Name != nil {
		*input.Name = strings.TrimSpace(*input.Name)
		if *input.Name == "" {
			return Role{}, ErrInvalidInput
		}
	}
	if len(input.Permissions) == 0 {
		return Role{}, ErrInvalidInput
	}
	for _, permission := range input.Permissions {
		if permission == constants.PermissionAdminsManage || permission == constants.PermissionRolesManage {
			return Role{}, ErrInvalidInput
		}
	}
	return s.repo.UpdateRole(ctx, actor.CompanyID, id, input)
}

func (s *Service) DeleteRole(ctx context.Context, actor rbac.Actor, id int64) error {
	if !actor.Can(constants.PermissionRolesManage, nil) {
		return ErrForbidden
	}
	return s.repo.DeleteRole(ctx, actor.CompanyID, id)
}
