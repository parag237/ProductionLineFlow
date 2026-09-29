package company

import (
	"context"
	"testing"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

type fakeRepository struct {
	called       bool
	passwordHash string
}

func (f *fakeRepository) CreateCompany(_ context.Context, _ Input, passwordHash string) (Result, error) {
	f.called = true
	f.passwordHash = passwordHash
	return Result{ID: 1, Slug: "acme", Name: "Acme"}, nil
}

func TestCreateRequiresPlatformPermission(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, 10)
	_, err := service.Create(context.Background(), rbac.PlatformActor{}, Input{Name: "Acme", Slug: "acme", SuperAdminName: "Admin", SuperAdminEmail: "admin@acme.test", SuperAdminPassword: "WarehouseDemo123!"})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if repository.called {
		t.Fatal("repository must not be called without permission")
	}
}

func TestCreateValidatesAndHashesPassword(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, 10)
	actor := rbac.PlatformActor{Permissions: []string{constants.PermissionCompaniesCreate}}
	result, err := service.Create(context.Background(), actor, Input{Name: " Acme ", Slug: "ACME", SuperAdminName: " Admin ", SuperAdminEmail: "admin@acme.test", SuperAdminPassword: "WarehouseDemo123!"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result.Slug != "acme" || !repository.called {
		t.Fatalf("unexpected result: %+v", result)
	}
	if repository.passwordHash == "WarehouseDemo123!" || len(repository.passwordHash) == 0 {
		t.Fatal("expected a password hash")
	}
}

func TestCreateRejectsShortPassword(t *testing.T) {
	repository := &fakeRepository{}
	service := NewService(repository, 10)
	actor := rbac.PlatformActor{Permissions: []string{constants.PermissionCompaniesCreate}}
	_, err := service.Create(context.Background(), actor, Input{Name: "Acme", Slug: "acme", SuperAdminName: "Admin", SuperAdminEmail: "admin@acme.test", SuperAdminPassword: "short"})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
