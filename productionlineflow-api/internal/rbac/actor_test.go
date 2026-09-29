package rbac

import "testing"

func TestActorCan(t *testing.T) {
	actor := Actor{
		Assignments: []Assignment{
			{Permission: "warehouse.view", WarehouseID: nil},
			{Permission: "warehouses.manage", WarehouseID: stringPtr("wh-1")},
		},
	}

	if !actor.Can("warehouse.view", nil) {
		t.Fatal("company-scoped permission should apply globally")
	}
	if !actor.Can("warehouses.manage", stringPtr("wh-1")) {
		t.Fatal("warehouse-scoped permission should match warehouse id")
	}
	if actor.Can("warehouses.manage", stringPtr("wh-2")) {
		t.Fatal("warehouse-scoped permission should not apply to other warehouse")
	}
}

func TestPlatformActorCanCreateCompanies(t *testing.T) {
	actor := PlatformActor{
		Role:        PlatformRoleProductOwner,
		Permissions: []string{PermissionCompaniesCreate},
	}

	if err := actor.MustHave(PermissionCompaniesCreate); err != nil {
		t.Fatalf("product owner should be able to create companies: %v", err)
	}
	if actor.Can("warehouses.manage") {
		t.Fatal("company creation permission should not grant tenant permissions")
	}

	unauthorized := PlatformActor{Role: PlatformRoleProductOwner}
	if unauthorized.Can(PermissionCompaniesCreate) {
		t.Fatal("product owner without companies.create should not create companies")
	}
}

func stringPtr(s string) *string {
	return &s
}
