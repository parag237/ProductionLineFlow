package rbac

import (
	"productionlineflow-api/internal/constants"
	"testing"
)

func TestActorCan(t *testing.T) {
	actor := Actor{
		Assignments: []Assignment{
			{Permission: "warehouse.view", WarehouseID: nil},
			{Permission: "warehouses.manage", WarehouseID: int64Ptr(1)},
		},
	}

	if !actor.Can("warehouse.view", nil) {
		t.Fatal("company-scoped permission should apply globally")
	}
	if !actor.Can("warehouses.manage", int64Ptr(1)) {
		t.Fatal("warehouse-scoped permission should match warehouse id")
	}
	if actor.Can("warehouses.manage", int64Ptr(2)) {
		t.Fatal("warehouse-scoped permission should not apply to other warehouse")
	}
}

func TestPlatformActorCanCreateCompanies(t *testing.T) {
	actor := PlatformActor{
		Role:        constants.PlatformRoleProductOwner,
		Permissions: DefaultPlatformRolePermissions(constants.PlatformRoleProductOwner),
	}

	if err := actor.MustHave(constants.PermissionCompaniesCreate); err != nil {
		t.Fatalf("product owner should be able to create companies: %v", err)
	}
	if actor.Can("warehouses.manage") {
		t.Fatal("company creation permission should not grant tenant permissions")
	}

	unauthorized := PlatformActor{Role: constants.PlatformRoleProductOwner}
	if unauthorized.Can(constants.PermissionCompaniesCreate) {
		t.Fatal("product owner without companies.create should not create companies")
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
