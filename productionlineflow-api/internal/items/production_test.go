package items

import (
	"context"
	"errors"
	"testing"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

type productionTestRepository struct {
	ProductionRepository
	companyID int64
	userID    int64
	input     RunInput
	created   bool
}

func (r *productionTestRepository) CreateRun(_ context.Context, companyID, userID int64, input RunInput) (ProductionRun, error) {
	r.companyID = companyID
	r.userID = userID
	r.input = input
	r.created = true
	return ProductionRun{ItemID: input.ItemID, TrackingMode: input.TrackingMode, Quantity: input.Quantity}, nil
}

func TestCreateBatchRequiresExecuteAndUsesActorScope(t *testing.T) {
	repository := &productionTestRepository{}
	service := NewProductionService(repository)
	input := RunInput{ItemID: 8, TrackingMode: "batch", Quantity: 5}
	viewer := rbac.Actor{CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionProductionView}}}
	if _, err := service.Create(context.Background(), viewer, input); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if repository.created {
		t.Fatal("unauthorized production run reached repository")
	}

	operator := rbac.Actor{UserID: 7, CompanyID: 34, Assignments: []rbac.Assignment{{Permission: constants.PermissionProductionExecute}}}
	if _, err := service.Create(context.Background(), operator, input); err != nil {
		t.Fatalf("create batch run: %v", err)
	}
	if repository.companyID != 34 || repository.userID != 7 || repository.input.Quantity != 5 {
		t.Fatalf("unexpected production scope/input: %#v", repository)
	}
}

func TestCreateUnitRunValidatesSerialNumbers(t *testing.T) {
	repository := &productionTestRepository{}
	service := NewProductionService(repository)
	operator := rbac.Actor{CompanyID: 12, Assignments: []rbac.Assignment{{Permission: constants.PermissionProductionExecute}}}
	invalid := []RunInput{
		{ItemID: 1, TrackingMode: "unit", Quantity: 2, SerialNumbers: []string{"A"}},
		{ItemID: 1, TrackingMode: "unit", Quantity: 2, SerialNumbers: []string{"A", "a"}},
		{ItemID: 1, TrackingMode: "batch", Quantity: 2, SerialNumbers: []string{"A", "B"}},
	}
	for _, input := range invalid {
		if _, err := service.Create(context.Background(), operator, input); !errors.Is(err, ErrInvalidTracking) {
			t.Fatalf("expected invalid tracking, got %v", err)
		}
	}
	if repository.created {
		t.Fatal("invalid run reached repository")
	}

	input := RunInput{ItemID: 1, TrackingMode: "unit", Quantity: 2, SerialNumbers: []string{" A-1 ", "B-2"}}
	if _, err := service.Create(context.Background(), operator, input); err != nil {
		t.Fatalf("create unit run: %v", err)
	}
	if repository.input.SerialNumbers[0] != "A-1" {
		t.Fatalf("serial number was not normalized: %#v", repository.input.SerialNumbers)
	}
}
