package items

import (
	"context"
	"errors"
	"testing"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

type testRepository struct {
	Repository
	companyID int64
	created   bool
	input     Input
}

func (r *testRepository) CreateItem(_ context.Context, companyID int64, input Input) (Item, error) {
	r.companyID = companyID
	r.created = true
	r.input = input
	return Item{ID: 1, Name: input.Name, SKU: input.SKU, IsActive: true}, nil
}

func TestCreateRequiresManageAndScopesToActorCompany(t *testing.T) {
	repository := &testRepository{}
	service := NewService(repository)
	input := Input{Name: "  Widget ", SKU: " SKU-1 ", Steps: []StepInput{{Title: "  Cut "}}}
	viewer := rbac.Actor{CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemsView}}}
	if _, err := service.Create(context.Background(), viewer, input); err != ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if repository.created {
		t.Fatal("unauthorized create reached repository")
	}

	manager := rbac.Actor{CompanyID: 34, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemsManage}}}
	if _, err := service.Create(context.Background(), manager, input); err != nil {
		t.Fatalf("create item: %v", err)
	}
	if repository.companyID != 34 || repository.input.Name != "Widget" || repository.input.SKU != "SKU-1" || repository.input.Steps[0].Title != "Cut" {
		t.Fatalf("unexpected persisted input: company=%d input=%#v", repository.companyID, repository.input)
	}
}

func TestCreateRejectsItemsWithoutValidSteps(t *testing.T) {
	repository := &testRepository{}
	service := NewService(repository)
	actor := rbac.Actor{CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemsManage}}}
	for _, input := range []Input{
		{Name: "No steps"},
		{Name: "Bad step", Steps: []StepInput{{Title: " "}}},
	} {
		if _, err := service.Create(context.Background(), actor, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	}
	if repository.created {
		t.Fatal("invalid item reached repository")
	}
}
