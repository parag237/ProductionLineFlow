package domain

import (
	"warehouse-api/internal/fsm"
)

var WarehouseLifecycle = fsm.Definition{
	Type:    "warehouse_lifecycle",
	Initial: "draft",
	Terminal: map[fsm.State]bool{"archived": true},
	Transitions: []fsm.Transition{
		{From: "draft", Event: "ACTIVATE", To: "active", Permission: "warehouses.manage"},
		{From: "active", Event: "SUSPEND", To: "suspended", Permission: "warehouses.manage"},
		{From: "suspended", Event: "REACTIVATE", To: "active", Permission: "warehouses.manage"},
		{From: "active", Event: "ARCHIVE", To: "archived", Permission: "warehouses.manage"},
		{From: "suspended", Event: "ARCHIVE", To: "archived", Permission: "warehouses.manage"},
	},
}
