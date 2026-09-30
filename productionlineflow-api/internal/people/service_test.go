package people

import (
	"context"
	"testing"

	"productionlineflow-api/internal/auth"
	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

type testRepository struct {
	Repository
	created      bool
	createInput  CreateInput
	policy       AssignmentPolicy
	passwordHash string
}

func (r *testRepository) CreatePerson(_ context.Context, _, _ int64, input CreateInput, hash string, policy AssignmentPolicy) (Person, error) {
	r.created = true
	r.createInput = input
	r.policy = policy
	r.passwordHash = hash
	if hash == "" {
		return Person{}, ErrInvalidInput
	}
	return Person{ID: 1, Name: input.Name, Email: input.Email}, nil
}
func (r *testRepository) ListPermissionCatalog(context.Context) ([]Permission, error) {
	return nil, nil
}

func TestCreatePersonValidatesBeforePersistence(t *testing.T) {
	repo := &testRepository{}
	service := NewService(repo, 8)
	actor := rbac.Actor{UserID: 4, CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionUsersCreate}}}
	if _, err := service.CreatePerson(context.Background(), actor, CreateInput{Name: "", Email: "bad", Password: "short"}); err != ErrInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if repo.created {
		t.Fatal("invalid input reached repository")
	}
	created, err := service.CreatePerson(context.Background(), actor, CreateInput{Name: "  Casey User ", Email: "casey@company.test", Password: "a-strong-password"})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	if created.Name != "Casey User" || !repo.policy.CompanyUser {
		t.Fatalf("unexpected create result/policy: %#v %#v", created, repo.policy)
	}
	if repo.passwordHash == repo.createInput.Password || !auth.VerifyPassword(repo.createInput.Password, repo.passwordHash) {
		t.Fatal("person password was not stored as a verifiable hash")
	}
	if repo.createInput.Password != "a-strong-password" {
		t.Fatal("repository should receive input for hashing workflow")
	}
}

func TestManagerCanOnlyCreateWithManagedWarehouseWorkerAssignment(t *testing.T) {
	repo := &testRepository{}
	service := NewService(repo, 8)
	warehouseID := int64(9)
	actor := rbac.Actor{UserID: 4, CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionWarehouseMembersManage, WarehouseID: &warehouseID}}}
	input := CreateInput{Name: "Worker", Email: "worker@company.test", Password: "a-strong-password", Assignments: []AssignmentInput{{RoleID: 2, WarehouseID: &warehouseID}}}
	if _, err := service.CreatePerson(context.Background(), actor, input); err != nil {
		t.Fatalf("manager worker create: %v", err)
	}
	if repo.policy.CompanyUser || len(repo.policy.Warehouses) != 1 || repo.policy.Warehouses[0] != warehouseID {
		t.Fatalf("unexpected manager policy: %#v", repo.policy)
	}
}

func TestCustomRoleRejectsProtectedPermissions(t *testing.T) {
	if validRoleInput("Auditor", "company", []string{constants.PermissionUsersCreate, constants.PermissionAdminsManage}) {
		t.Fatal("custom role accepted protected permission")
	}
	if validRoleInput("Auditor", "invalid", []string{"warehouse.view"}) {
		t.Fatal("custom role accepted invalid scope")
	}
}
