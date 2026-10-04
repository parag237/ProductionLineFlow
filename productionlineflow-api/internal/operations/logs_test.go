package operations

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

type fakeRepository struct {
	warehouses       []WarehouseOption
	items            []ItemOption
	performers       []PerformerOption
	entries          []Entry
	created          Entry
	lastCompanyID    int64
	lastRecorderID   int64
	lastWarehouseIDs []int64
	lastCompanyWide  bool
	lastListIDs      []int64
	lastListWide     bool
	lastCreateInput  Input
	listCalls        int
	createCalls      int
	optionsCalls     int
}

func (r *fakeRepository) ListWarehouses(_ context.Context, companyID int64, warehouseIDs []int64, companyWide bool) ([]WarehouseOption, error) {
	r.lastCompanyID = companyID
	r.lastWarehouseIDs = warehouseIDs
	r.lastCompanyWide = companyWide
	r.optionsCalls++
	return r.warehouses, nil
}

func (r *fakeRepository) ListCatalog(context.Context, int64) ([]ItemOption, error) {
	return r.items, nil
}

func (r *fakeRepository) ListPerformers(context.Context, int64, int64) ([]PerformerOption, error) {
	return r.performers, nil
}

func (r *fakeRepository) ListEntries(_ context.Context, companyID int64, warehouseIDs []int64, companyWide bool, _ string) ([]Entry, error) {
	r.lastCompanyID = companyID
	r.lastListIDs = warehouseIDs
	r.lastListWide = companyWide
	r.listCalls++
	return r.entries, nil
}

func (r *fakeRepository) CreateEntry(_ context.Context, companyID, recorderID int64, input Input) (Entry, error) {
	r.lastCompanyID = companyID
	r.lastRecorderID = recorderID
	r.lastCreateInput = input
	r.createCalls++
	return r.created, nil
}

func TestCreateUsesActorAsRecorderAndSelectedPerformer(t *testing.T) {
	repository := &fakeRepository{created: Entry{ID: 4}}
	service := NewService(repository)
	warehouseID := int64(12)
	actor := rbac.Actor{UserID: 8, CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID}}}
	input := Input{WarehouseID: warehouseID, WorkDate: todayDate(), ItemID: 21, StepID: 34, Quantity: 5, PerformedBy: 9}

	entry, err := service.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if entry.ID != 4 || repository.lastCompanyID != actor.CompanyID || repository.lastRecorderID != actor.UserID {
		t.Fatalf("unexpected created entry or identity: entry=%+v repository=%+v", entry, repository)
	}
	if repository.lastCreateInput.PerformedBy != 9 {
		t.Fatalf("expected selected performer 9, got %d", repository.lastCreateInput.PerformedBy)
	}
}

func TestCreateRejectsUnassignedWarehouseAndInvalidInput(t *testing.T) {
	warehouseID := int64(12)
	service := NewService(&fakeRepository{})
	actor := rbac.Actor{UserID: 8, CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID}}}
	input := Input{WarehouseID: 13, WorkDate: "2026-10-04", ItemID: 21, StepID: 34, Quantity: 5, PerformedBy: 9}
	if _, err := service.Create(context.Background(), actor, input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for another warehouse, got %v", err)
	}

	input.WarehouseID = warehouseID
	input.WorkDate = "2026-02-30"
	if _, err := service.Create(context.Background(), actor, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid date error, got %v", err)
	}
	input.WorkDate = "2026-10-04"
	input.Quantity = 0
	if _, err := service.Create(context.Background(), actor, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid quantity error, got %v", err)
	}
}

func TestListRequiresWarehouseViewPermission(t *testing.T) {
	repository := &fakeRepository{entries: []Entry{{ID: 2}}}
	service := NewService(repository)
	warehouseID := int64(12)
	actor := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsView, WarehouseID: &warehouseID}}}

	entries, err := service.List(context.Background(), actor, &warehouseID, todayDate())
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one listed entry, got entries=%v err=%v", entries, err)
	}
	otherWarehouseID := int64(13)
	if _, err := service.List(context.Background(), actor, &otherWarehouseID, todayDate()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for another warehouse, got %v", err)
	}
	if _, err := service.List(context.Background(), actor, &warehouseID, "2026-13-01"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid date error, got %v", err)
	}
}

func TestCreatePermissionDoesNotGrantReadOrSelectionOptions(t *testing.T) {
	warehouseID := int64(12)
	repository := &fakeRepository{warehouses: []WarehouseOption{{ID: warehouseID, Name: "North"}}}
	service := NewService(repository)
	createOnly := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID}}}

	if _, err := service.List(context.Background(), createOnly, &warehouseID, todayDate()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected create-only actor to be denied reads, got %v", err)
	}
	if repository.listCalls != 0 {
		t.Fatal("create-only actor reached the entry repository")
	}
	viewOnly := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsView, WarehouseID: &warehouseID}}}
	if _, err := service.Options(context.Background(), viewOnly, &warehouseID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected view-only actor to be denied create options, got %v", err)
	}
}

func TestListAllWarehousesUsesOnlyViewableScope(t *testing.T) {
	firstWarehouseID := int64(12)
	secondWarehouseID := int64(14)
	createOnlyWarehouseID := int64(18)
	repository := &fakeRepository{entries: []Entry{{ID: 2}}}
	service := NewService(repository)
	manager := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{
		{Permission: constants.PermissionOperationLogsView, WarehouseID: &firstWarehouseID},
		{Permission: constants.PermissionOperationLogsView, WarehouseID: &secondWarehouseID},
		{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &createOnlyWarehouseID},
	}}

	if _, err := service.List(context.Background(), manager, nil, todayDate()); err != nil {
		t.Fatalf("List all returned error: %v", err)
	}
	sort.Slice(repository.lastListIDs, func(left, right int) bool { return repository.lastListIDs[left] < repository.lastListIDs[right] })
	if !reflect.DeepEqual(repository.lastListIDs, []int64{firstWarehouseID, secondWarehouseID}) || repository.lastListWide {
		t.Fatalf("all-warehouse list exceeded view permissions: ids=%v companyWide=%v", repository.lastListIDs, repository.lastListWide)
	}

	admin := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsView}}}
	if _, err := service.List(context.Background(), admin, nil, todayDate()); err != nil {
		t.Fatalf("company-wide List all returned error: %v", err)
	}
	if !repository.lastListWide {
		t.Fatal("expected company admin to list all company warehouses")
	}
}

func TestOnlyDateOverridePermissionAllowsNonCurrentWorkDate(t *testing.T) {
	warehouseID := int64(12)
	nonCurrentDate := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	input := Input{WarehouseID: warehouseID, WorkDate: nonCurrentDate, ItemID: 21, StepID: 34, Quantity: 5, PerformedBy: 9}
	repository := &fakeRepository{}
	service := NewService(repository)
	manager := rbac.Actor{UserID: 8, CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID}}}
	if _, err := service.Create(context.Background(), manager, input); !errors.Is(err, ErrDateOverrideForbidden) {
		t.Fatalf("expected manager date override to be denied, got %v", err)
	}
	if repository.createCalls != 0 {
		t.Fatal("manager date override reached the repository")
	}

	admin := rbac.Actor{UserID: 8, CompanyID: 3, Assignments: []rbac.Assignment{
		{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID},
		{Permission: constants.PermissionOperationLogsDateOverride},
	}}
	if _, err := service.Create(context.Background(), admin, input); err != nil {
		t.Fatalf("admin date override returned error: %v", err)
	}

	superAdmin := rbac.Actor{UserID: 8, CompanyID: 3, Assignments: []rbac.Assignment{
		{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID},
		{Permission: constants.PermissionAdminsManage},
	}}
	if _, err := service.Create(context.Background(), superAdmin, input); err != nil {
		t.Fatalf("super admin date override returned error: %v", err)
	}
}

func TestOptionsRestrictManagerWarehousesAndCompanyAdmins(t *testing.T) {
	warehouseID := int64(12)
	repository := &fakeRepository{warehouses: []WarehouseOption{{ID: warehouseID, Name: "North"}}, items: []ItemOption{{ID: 5}}, performers: []PerformerOption{{ID: 9}}}
	service := NewService(repository)
	manager := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{
		{Permission: constants.PermissionOperationLogsView, WarehouseID: &warehouseID},
		{Permission: constants.PermissionOperationLogsCreate, WarehouseID: &warehouseID},
	}}

	options, err := service.Options(context.Background(), manager, nil)
	if err != nil {
		t.Fatalf("Options returned error: %v", err)
	}
	if !reflect.DeepEqual(repository.lastWarehouseIDs, []int64{warehouseID}) || repository.lastCompanyWide {
		t.Fatalf("manager options were not warehouse scoped: ids=%v companyWide=%v", repository.lastWarehouseIDs, repository.lastCompanyWide)
	}
	selectedWarehouse := int64(13)
	if _, err := service.Options(context.Background(), manager, &selectedWarehouse); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for unassigned warehouse, got %v", err)
	}
	selectedWarehouse = warehouseID
	options, err = service.Options(context.Background(), manager, &selectedWarehouse)
	if err != nil || len(options.Items) != 1 || len(options.Performers) != 1 {
		t.Fatalf("expected create options for assigned warehouse, got options=%+v err=%v", options, err)
	}

	admin := rbac.Actor{CompanyID: 3, Assignments: []rbac.Assignment{{Permission: constants.PermissionOperationLogsView}, {Permission: constants.PermissionOperationLogsCreate}}}
	if _, err := service.Options(context.Background(), admin, nil); err != nil {
		t.Fatalf("company admin options returned error: %v", err)
	}
	if !repository.lastCompanyWide {
		t.Fatal("expected company-wide options for admin")
	}
	selectedWarehouse = 13
	if _, err := service.Options(context.Background(), admin, &selectedWarehouse); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for unknown company warehouse, got %v", err)
	}
}
