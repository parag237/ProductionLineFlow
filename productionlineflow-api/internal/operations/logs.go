package operations

import (
	"context"
	"errors"
	"time"

	"productionlineflow-api/internal/constants"
	"productionlineflow-api/internal/rbac"
)

var (
	ErrForbidden    = errors.New("operation log permission denied")
	ErrInvalidInput = errors.New("invalid operation log input")
	ErrNotFound     = errors.New("operation log not found")
)

type WarehouseOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type StepOption struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type ItemOption struct {
	ID    int64        `json:"id"`
	Name  string       `json:"name"`
	Steps []StepOption `json:"steps"`
}

type PerformerOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Options struct {
	Warehouses []WarehouseOption `json:"warehouses"`
	Items      []ItemOption      `json:"items"`
	Performers []PerformerOption `json:"performers"`
}

type Entry struct {
	ID            int64  `json:"id"`
	WarehouseID   int64  `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	WorkDate      string `json:"work_date"`
	ItemID        int64  `json:"item_id"`
	ItemName      string `json:"item_name"`
	StepID        int64  `json:"step_id"`
	StepTitle     string `json:"step_title"`
	Quantity      int64  `json:"quantity"`
	PerformedBy   int64  `json:"performed_by"`
	PerformerName string `json:"performer_name"`
	RecordedBy    int64  `json:"recorded_by"`
	CreatedAt     string `json:"created_at"`
}

type Input struct {
	WarehouseID int64  `json:"warehouse_id"`
	WorkDate    string `json:"work_date"`
	ItemID      int64  `json:"item_id"`
	StepID      int64  `json:"step_id"`
	Quantity    int64  `json:"quantity"`
	PerformedBy int64  `json:"performed_by"`
}

type Repository interface {
	ListWarehouses(context.Context, int64, []int64, bool) ([]WarehouseOption, error)
	ListCatalog(context.Context, int64) ([]ItemOption, error)
	ListPerformers(context.Context, int64, int64) ([]PerformerOption, error)
	ListEntries(context.Context, int64, int64, string) ([]Entry, error)
	CreateEntry(context.Context, int64, int64, Input) (Entry, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Options(ctx context.Context, actor rbac.Actor, warehouseID *int64) (Options, error) {
	warehouseIDs, companyWide := allowedWarehouses(actor)
	if !companyWide && len(warehouseIDs) == 0 {
		return Options{}, ErrForbidden
	}
	warehouses, err := s.repository.ListWarehouses(ctx, actor.CompanyID, warehouseIDs, companyWide)
	if err != nil {
		return Options{}, err
	}
	options := Options{Warehouses: warehouses, Items: []ItemOption{}, Performers: []PerformerOption{}}
	if warehouseID == nil {
		return options, nil
	}
	warehouseExists := false
	for _, warehouse := range warehouses {
		if warehouse.ID == *warehouseID {
			warehouseExists = true
			break
		}
	}
	if !warehouseExists {
		return Options{}, ErrForbidden
	}
	if !actor.Can(constants.PermissionOperationLogsCreate, warehouseID) {
		return Options{}, ErrForbidden
	}
	options.Items, err = s.repository.ListCatalog(ctx, actor.CompanyID)
	if err != nil {
		return Options{}, err
	}
	options.Performers, err = s.repository.ListPerformers(ctx, actor.CompanyID, *warehouseID)
	if err != nil {
		return Options{}, err
	}
	return options, nil
}

func (s *Service) List(ctx context.Context, actor rbac.Actor, warehouseID int64, workDate string) ([]Entry, error) {
	if !actor.Can(constants.PermissionOperationLogsView, &warehouseID) {
		return nil, ErrForbidden
	}
	if warehouseID < 1 || !validDate(workDate) {
		return nil, ErrInvalidInput
	}
	return s.repository.ListEntries(ctx, actor.CompanyID, warehouseID, workDate)
}

func (s *Service) Create(ctx context.Context, actor rbac.Actor, input Input) (Entry, error) {
	if !actor.Can(constants.PermissionOperationLogsCreate, &input.WarehouseID) {
		return Entry{}, ErrForbidden
	}
	if input.WarehouseID < 1 || input.ItemID < 1 || input.StepID < 1 || input.PerformedBy < 1 || input.Quantity < 1 || !validDate(input.WorkDate) {
		return Entry{}, ErrInvalidInput
	}
	return s.repository.CreateEntry(ctx, actor.CompanyID, actor.UserID, input)
}

func allowedWarehouses(actor rbac.Actor) ([]int64, bool) {
	permissions := []string{constants.PermissionOperationLogsView, constants.PermissionOperationLogsCreate}
	ids := map[int64]struct{}{}
	for _, permission := range permissions {
		if actor.Can(permission, nil) {
			return nil, true
		}
		for _, assignment := range actor.Assignments {
			if assignment.Permission == permission && assignment.WarehouseID != nil {
				ids[*assignment.WarehouseID] = struct{}{}
			}
		}
	}
	warehouseIDs := make([]int64, 0, len(ids))
	for id := range ids {
		warehouseIDs = append(warehouseIDs, id)
	}
	return warehouseIDs, false
}

func validDate(value string) bool {
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}
