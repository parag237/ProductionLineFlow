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
	companyID      int64
	created        bool
	updated        bool
	categoryExists bool
	input          Input
}

func (r *testRepository) CreateItem(_ context.Context, companyID int64, input Input) (Item, error) {
	r.companyID = companyID
	r.created = true
	r.input = input
	return Item{ID: 1, Name: input.Name, SKU: input.SKU, UnitOfMeasure: input.UnitOfMeasure, IsActive: true}, nil
}

func (r *testRepository) UpdateItem(_ context.Context, companyID, _ int64, input Input) (Item, error) {
	r.companyID = companyID
	r.updated = true
	r.input = input
	return Item{ID: 1, Name: input.Name, UnitOfMeasure: input.UnitOfMeasure, CategoryID: input.CategoryID, IsActive: true}, nil
}

func (r *testRepository) CategoryExists(_ context.Context, companyID, categoryID int64) (bool, error) {
	return r.categoryExists && categoryID > 0, nil
}

func TestCreateRequiresManageAndScopesToActorCompany(t *testing.T) {
	repository := &testRepository{categoryExists: true}
	service := NewService(repository)
	input := Input{Name: "  Widget ", SKU: " SKU-1 ", UnitOfMeasure: " kg ", CategoryID: 5, Steps: []StepInput{{Title: "  Cut "}}}
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
	if repository.companyID != 34 || repository.input.Name != "Widget" || repository.input.SKU != "SKU-1" || repository.input.UnitOfMeasure != "kg" || repository.input.CategoryID != 5 || repository.input.Steps[0].Title != "Cut" {
		t.Fatalf("unexpected persisted input: company=%d input=%#v", repository.companyID, repository.input)
	}
}

func TestCreateRejectsItemsWithoutValidSteps(t *testing.T) {
	repository := &testRepository{categoryExists: true}
	service := NewService(repository)
	actor := rbac.Actor{CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemsManage}}}
	for _, input := range []Input{
		{Name: "No steps"},
		{Name: "Bad step", Steps: []StepInput{{Title: " "}}},
		{Name: "No unit", Steps: []StepInput{{Title: "Cut"}}},
		{Name: "Long unit", UnitOfMeasure: "measure-measure-measure-measure-measure-too-long", Steps: []StepInput{{Title: "Cut"}}},
	} {
		if _, err := service.Create(context.Background(), actor, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	}
	if repository.created {
		t.Fatal("invalid item reached repository")
	}
}

func TestCreateRejectsCategoryOutsideCompany(t *testing.T) {
	repository := &testRepository{}
	service := NewService(repository)
	actor := rbac.Actor{CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemsManage}}}
	input := Input{Name: "Widget", UnitOfMeasure: "pieces", CategoryID: 8, Steps: []StepInput{{Title: "Pack"}}}
	if _, err := service.Create(context.Background(), actor, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid category error, got %v", err)
	}
	if repository.created {
		t.Fatal("item with unverified category reached repository")
	}
}

func TestUpdatePersistsSelectedCategory(t *testing.T) {
	repository := &testRepository{categoryExists: true}
	service := NewService(repository)
	actor := rbac.Actor{CompanyID: 22, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemsManage}}}
	input := Input{Name: "Widget", UnitOfMeasure: "pieces", CategoryID: 19, Steps: []StepInput{{Title: "Pack"}}}
	item, err := service.Update(context.Background(), actor, 4, input)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if !repository.updated || repository.companyID != 22 || repository.input.CategoryID != 19 || item.CategoryID != 19 {
		t.Fatalf("selected category was not preserved: repository=%+v item=%+v", repository, item)
	}
}
