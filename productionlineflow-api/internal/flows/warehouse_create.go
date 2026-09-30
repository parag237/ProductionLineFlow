package flows

import (
	"context"
	"productionlineflow-api/internal/fsm"
	"productionlineflow-api/internal/ui"
)

var WarehouseCreate = fsm.Definition{
	Type:     "warehouse_create",
	Initial:  "details",
	Terminal: map[fsm.State]bool{"done": true, "cancelled": true},
	Transitions: []fsm.Transition{
		{From: "details", Event: "SUBMIT_DETAILS", To: "review", Permission: "warehouses.manage"},
		{From: "review", Event: "BACK", To: "details", Permission: "warehouses.manage"},
		{From: "review", Event: "CONFIRM", To: "done", Permission: "warehouses.manage"},
		{From: "details", Event: "CANCEL", To: "cancelled", Permission: "warehouses.manage"},
	},
	Screens: map[fsm.State]fsm.ScreenBuilder{
		"details": func(ctx context.Context, s *fsm.Session) any {
			return ui.Screen{
				Name:          "warehouse.create.details",
				SchemaVersion: 1,
				Title:         "Warehouse details",
				Components: []ui.Component{{
					Type: "form",
					ID:   "details_form",
					Fields: []ui.Field{
						{Type: "text_input", Name: "name", Label: "Warehouse name", Required: true, MinLength: 2},
						{Type: "select", Name: "type_id", Label: "Type", Required: true, OptionsSource: "warehouse_types"},
					},
				}},
				Actions: []ui.Action{
					{ID: "cancel", Label: "Cancel", Event: "CANCEL", Style: "secondary"},
					{ID: "next", Label: "Next", Event: "SUBMIT_DETAILS", Style: "primary", Submits: "details_form"},
				},
			}
		},
		"review": func(ctx context.Context, s *fsm.Session) any {
			return ui.Screen{
				Name:          "warehouse.create.review",
				SchemaVersion: 1,
				Title:         "Review warehouse",
				Components:    []ui.Component{{Type: "card", Text: "Review the warehouse details before creating it."}},
				Actions: []ui.Action{
					{ID: "back", Label: "Back", Event: "BACK", Style: "secondary"},
					{ID: "confirm", Label: "Create warehouse", Event: "CONFIRM", Style: "primary"},
				},
			}
		},
		"done": func(ctx context.Context, s *fsm.Session) any {
			return ui.Screen{
				Name:          "warehouse.create.done",
				SchemaVersion: 1,
				Title:         "Warehouse created",
				Components:    []ui.Component{{Type: "banner", Text: "The warehouse has been created."}},
				Actions:       []ui.Action{{ID: "done", Label: "Done", Event: "DONE", Style: "primary"}},
			}
		},
	},
}
