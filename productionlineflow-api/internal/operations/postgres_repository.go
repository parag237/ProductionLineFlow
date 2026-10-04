package operations

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListWarehouses(ctx context.Context, companyID int64, warehouseIDs []int64, companyWide bool) ([]WarehouseOption, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name FROM warehouses
		WHERE company_id = $1 AND ($3::BOOLEAN OR id = ANY($2::BIGINT[]))
		ORDER BY name, id
	`, companyID, warehouseIDs, companyWide)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	warehouses := make([]WarehouseOption, 0)
	for rows.Next() {
		var warehouse WarehouseOption
		if err := rows.Scan(&warehouse.ID, &warehouse.Name); err != nil {
			return nil, err
		}
		warehouses = append(warehouses, warehouse)
	}
	return warehouses, rows.Err()
}

func (r *PostgresRepository) ListCatalog(ctx context.Context, companyID int64) ([]ItemOption, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.name, s.id, s.title
		FROM items i
	JOIN item_steps s ON s.company_id = i.company_id AND s.item_id = i.id
		WHERE i.company_id = $1 AND i.is_active
		ORDER BY i.name, i.id, s.position, s.id
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ItemOption, 0)
	indices := make(map[int64]int)
	for rows.Next() {
		var item ItemOption
		var step StepOption
		if err := rows.Scan(&item.ID, &item.Name, &step.ID, &step.Title); err != nil {
			return nil, err
		}
		index, exists := indices[item.ID]
		if !exists {
			item.Steps = make([]StepOption, 0, 1)
			index = len(items)
			indices[item.ID] = index
			items = append(items, item)
		}
		items[index].Steps = append(items[index].Steps, step)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) ListPerformers(ctx context.Context, companyID, warehouseID int64) ([]PerformerOption, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT u.id, u.name
		FROM users u
		JOIN user_role_assignments assignment ON assignment.company_id = u.company_id AND assignment.user_id = u.id
		JOIN roles role ON role.company_id = assignment.company_id AND role.id = assignment.role_id
		WHERE u.company_id = $1 AND u.is_active
		  AND assignment.warehouse_id = $2 AND role.slug = 'worker'
		ORDER BY u.name, u.id
	`, companyID, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	performers := make([]PerformerOption, 0)
	for rows.Next() {
		var performer PerformerOption
		if err := rows.Scan(&performer.ID, &performer.Name); err != nil {
			return nil, err
		}
		performers = append(performers, performer)
	}
	return performers, rows.Err()
}

func (r *PostgresRepository) ListEntries(ctx context.Context, companyID, warehouseID int64, workDate string) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT log.id, log.warehouse_id, warehouse.name, log.work_date::TEXT,
		       log.item_id, log.item_name, log.step_id, log.step_title, log.quantity,
		       log.performed_by, performer.name, log.recorded_by, log.created_at::TEXT
		FROM operation_work_logs log
		JOIN warehouses warehouse ON warehouse.company_id = log.company_id AND warehouse.id = log.warehouse_id
		JOIN users performer ON performer.company_id = log.company_id AND performer.id = log.performed_by
		WHERE log.company_id = $1 AND log.warehouse_id = $2 AND log.work_date = $3::DATE
		ORDER BY log.created_at DESC, log.id DESC
	`, companyID, warehouseID, workDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.WarehouseID, &entry.WarehouseName, &entry.WorkDate,
			&entry.ItemID, &entry.ItemName, &entry.StepID, &entry.StepTitle, &entry.Quantity,
			&entry.PerformedBy, &entry.PerformerName, &entry.RecordedBy, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *PostgresRepository) CreateEntry(ctx context.Context, companyID, recorderID int64, input Input) (Entry, error) {
	var entry Entry
	err := r.pool.QueryRow(ctx, `
		INSERT INTO operation_work_logs (
			company_id, warehouse_id, work_date, item_id, item_name,
			step_id, step_title, quantity, performed_by, recorded_by
		)
		SELECT $1, $2, $3::DATE, item.id, item.name,
		       step.id, step.title, $7, performer.id, $8
		FROM items item
		JOIN item_steps step ON step.company_id = item.company_id AND step.item_id = item.id
		JOIN warehouses warehouse ON warehouse.company_id = item.company_id AND warehouse.id = $2
		JOIN users performer ON performer.company_id = item.company_id AND performer.id = $6 AND performer.is_active
		WHERE item.company_id = $1 AND item.id = $4 AND item.is_active AND step.id = $5
		  AND EXISTS (
			SELECT 1
			FROM user_role_assignments assignment
			JOIN roles role ON role.company_id = assignment.company_id AND role.id = assignment.role_id
			WHERE assignment.company_id = item.company_id
			  AND assignment.user_id = performer.id
			  AND assignment.warehouse_id = warehouse.id
			  AND role.slug = 'worker'
		  )
		RETURNING id, warehouse_id,
		          (SELECT name FROM warehouses WHERE company_id = $1 AND id = $2),
		          work_date::TEXT, item_id, item_name, step_id, step_title, quantity,
		          performed_by, (SELECT name FROM users WHERE company_id = $1 AND id = $6),
		          recorded_by, created_at::TEXT
	`, companyID, input.WarehouseID, input.WorkDate, input.ItemID, input.StepID,
		input.PerformedBy, input.Quantity, recorderID).Scan(
		&entry.ID, &entry.WarehouseID, &entry.WarehouseName, &entry.WorkDate,
		&entry.ItemID, &entry.ItemName, &entry.StepID, &entry.StepTitle, &entry.Quantity,
		&entry.PerformedBy, &entry.PerformerName, &entry.RecordedBy, &entry.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrInvalidInput
	}
	return entry, err
}
