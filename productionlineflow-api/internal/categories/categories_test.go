package categories

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
	name      string
	created   bool
	deleted   bool
}

func (r *testRepository) List(_ context.Context, companyID int64) ([]Category, error) {
	r.companyID = companyID
	return []Category{{ID: 1, Name: "Raw material"}}, nil
}

func (r *testRepository) Create(_ context.Context, companyID int64, name string) (Category, error) {
	r.companyID = companyID
	r.name = name
	r.created = true
	return Category{ID: 1, Name: name}, nil
}

func (r *testRepository) Update(_ context.Context, companyID, _ int64, name string) (Category, error) {
	r.companyID = companyID
	r.name = name
	return Category{ID: 2, Name: name}, nil
}

func (r *testRepository) Delete(_ context.Context, companyID, _ int64) error {
	r.companyID = companyID
	r.deleted = true
	return nil
}

func TestViewPermissionListsCategoriesButCannotManage(t *testing.T) {
	repository := &testRepository{}
	service := NewService(repository)
	actor := rbac.Actor{CompanyID: 17, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemCategoriesView}}}
	if categories, err := service.List(context.Background(), actor); err != nil || len(categories) != 1 {
		t.Fatalf("List returned categories=%v err=%v", categories, err)
	}
	if repository.companyID != 17 {
		t.Fatalf("expected company scope 17, got %d", repository.companyID)
	}
	if _, err := service.Create(context.Background(), actor, "Raw"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected view-only create to be forbidden, got %v", err)
	}
}

func TestManagePermissionNormalizesAndScopesCategory(t *testing.T) {
	repository := &testRepository{}
	service := NewService(repository)
	actor := rbac.Actor{CompanyID: 31, Assignments: []rbac.Assignment{{Permission: constants.PermissionItemCategoriesManage}}}
	category, err := service.Create(context.Background(), actor, "  Packaging  ")
	if err != nil || category.Name != "Packaging" || repository.companyID != 31 {
		t.Fatalf("unexpected category create result: category=%+v repo=%+v err=%v", category, repository, err)
	}
	if err := service.Delete(context.Background(), actor, 1); err != nil || !repository.deleted {
		t.Fatalf("expected category delete, got deleted=%v err=%v", repository.deleted, err)
	}
	if _, err := service.Create(context.Background(), actor, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected blank category name to fail, got %v", err)
	}
}
