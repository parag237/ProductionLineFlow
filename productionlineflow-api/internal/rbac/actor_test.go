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

func stringPtr(s string) *string {
	return &s
}
