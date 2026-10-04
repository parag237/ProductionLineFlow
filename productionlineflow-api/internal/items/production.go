package items

import (
	"context"
	"errors"
	"strings"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var ErrInvalidTracking = errors.New("invalid production tracking input")

type RunStep struct {
	ID           int64  `json:"id"`
	Position     int    `json:"position"`
	Title        string `json:"title"`
	Instructions string `json:"instructions,omitempty"`
}

type ProductionUnit struct {
	ID           int64  `json:"id"`
	SerialNumber string `json:"serial_number"`
	Status       string `json:"status"`
}

type StepEvent struct {
	ID        int64  `json:"id"`
	RunStepID int64  `json:"run_step_id"`
	UnitID    *int64 `json:"unit_id,omitempty"`
	Event     string `json:"event"`
	ActorID   int64  `json:"actor_id"`
	Note      string `json:"note,omitempty"`
	CreatedAt string `json:"created_at"`
}

type ProductionRun struct {
	ID            int64            `json:"id"`
	ItemID        int64            `json:"item_id"`
	ItemName      string           `json:"item_name"`
	UnitOfMeasure string           `json:"unit_of_measure"`
	TrackingMode  string           `json:"tracking_mode"`
	Quantity      int              `json:"quantity"`
	Status        string           `json:"status"`
	CreatedBy     int64            `json:"created_by"`
	StartedAt     string           `json:"started_at"`
	CompletedAt   *string          `json:"completed_at,omitempty"`
	Steps         []RunStep        `json:"steps"`
	Units         []ProductionUnit `json:"units"`
	Events        []StepEvent      `json:"events"`
}

type RunInput struct {
	ItemID        int64    `json:"item_id"`
	TrackingMode  string   `json:"tracking_mode"`
	Quantity      int      `json:"quantity"`
	SerialNumbers []string `json:"serial_numbers"`
}

type StepCompletionInput struct {
	UnitID *int64 `json:"unit_id"`
	Note   string `json:"note"`
}

type ProductionRepository interface {
	ListRuns(context.Context, int64) ([]ProductionRun, error)
	GetRun(context.Context, int64, int64) (ProductionRun, error)
	CreateRun(context.Context, int64, int64, RunInput) (ProductionRun, error)
	CompleteStep(context.Context, int64, int64, int64, int64, StepCompletionInput) (ProductionRun, error)
}

type ProductionService struct{ repository ProductionRepository }

func NewProductionService(repository ProductionRepository) *ProductionService {
	return &ProductionService{repository: repository}
}

func (s *ProductionService) List(ctx context.Context, actor rbac.Actor) ([]ProductionRun, error) {
	if !actor.Can(constants.PermissionProductionView, nil) && !actor.Can(constants.PermissionProductionExecute, nil) {
		return nil, ErrForbidden
	}
	return s.repository.ListRuns(ctx, actor.CompanyID)
}

func (s *ProductionService) Get(ctx context.Context, actor rbac.Actor, id int64) (ProductionRun, error) {
	if !actor.Can(constants.PermissionProductionView, nil) && !actor.Can(constants.PermissionProductionExecute, nil) {
		return ProductionRun{}, ErrForbidden
	}
	return s.repository.GetRun(ctx, actor.CompanyID, id)
}

func (s *ProductionService) Create(ctx context.Context, actor rbac.Actor, input RunInput) (ProductionRun, error) {
	if !actor.Can(constants.PermissionProductionExecute, nil) {
		return ProductionRun{}, ErrForbidden
	}
	if input.ItemID <= 0 || input.Quantity <= 0 {
		return ProductionRun{}, ErrInvalidTracking
	}
	switch input.TrackingMode {
	case "batch":
		if len(input.SerialNumbers) != 0 {
			return ProductionRun{}, ErrInvalidTracking
		}
	case "unit":
		if len(input.SerialNumbers) != input.Quantity {
			return ProductionRun{}, ErrInvalidTracking
		}
		seen := make(map[string]struct{}, len(input.SerialNumbers))
		for index, serial := range input.SerialNumbers {
			serial = strings.TrimSpace(serial)
			if serial == "" || len(serial) > 120 {
				return ProductionRun{}, ErrInvalidTracking
			}
			if _, exists := seen[strings.ToLower(serial)]; exists {
				return ProductionRun{}, ErrInvalidTracking
			}
			seen[strings.ToLower(serial)] = struct{}{}
			input.SerialNumbers[index] = serial
		}
	default:
		return ProductionRun{}, ErrInvalidTracking
	}
	return s.repository.CreateRun(ctx, actor.CompanyID, actor.UserID, input)
}

func (s *ProductionService) CompleteStep(ctx context.Context, actor rbac.Actor, runID, stepID int64, input StepCompletionInput) (ProductionRun, error) {
	if !actor.Can(constants.PermissionProductionExecute, nil) {
		return ProductionRun{}, ErrForbidden
	}
	if runID <= 0 || stepID <= 0 || (input.UnitID != nil && *input.UnitID <= 0) || len(input.Note) > 1000 {
		return ProductionRun{}, ErrInvalidTracking
	}
	input.Note = strings.TrimSpace(input.Note)
	return s.repository.CompleteStep(ctx, actor.CompanyID, actor.UserID, runID, stepID, input)
}
