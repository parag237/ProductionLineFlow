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
		SELECT i.id, i.name, category.name, i.unit_of_measure, s.id, s.title
		FROM items i
		JOIN item_categories category ON category.company_id = i.company_id AND category.id = i.category_id
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
		if err := rows.Scan(&item.ID, &item.Name, &item.CategoryName, &item.UnitOfMeasure, &step.ID, &step.Title); err != nil {
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

func (r *PostgresRepository) ListEntries(ctx context.Context, companyID int64, warehouseIDs []int64, companyWide bool, workDate string) ([]Entry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT log.id, log.warehouse_id, warehouse.name, log.work_date::TEXT,
		       log.item_id, log.item_name, log.category_name, log.step_id, log.step_title, log.unit_of_measure, log.quantity,
		       log.performed_by, performer.name, log.recorded_by, log.created_at::TEXT
		FROM operation_work_logs log
		JOIN warehouses warehouse ON warehouse.company_id = log.company_id AND warehouse.id = log.warehouse_id
		JOIN users performer ON performer.company_id = log.company_id AND performer.id = log.performed_by
		WHERE log.company_id = $1 AND ($2::BOOLEAN OR log.warehouse_id = ANY($3::BIGINT[])) AND log.work_date = $4::DATE
		ORDER BY log.created_at DESC, log.id DESC
	`, companyID, companyWide, warehouseIDs, workDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

const analysisFilter = `company_id = $1 AND ($2::BOOLEAN OR warehouse_id = ANY($3::BIGINT[])) AND work_date BETWEEN $4::DATE AND $5::DATE`

func (r *PostgresRepository) Analyze(ctx context.Context, companyID int64, warehouseIDs []int64, companyWide bool, fromDate, toDate string) (Analysis, error) {
	analysis := Analysis{
		Units:      []UnitTotal{},
		Daily:      []DailyTotal{},
		Categories: []CategoryTotal{},
		Items:      []ItemTotal{},
		Steps:      []StepTotal{},
		Performers: []PerformerTotal{},
	}
	args := []any{companyID, companyWide, warehouseIDs, fromDate, toDate}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT item_id), COUNT(DISTINCT (item_id, step_id)), COUNT(DISTINCT performed_by)
		FROM operation_work_logs WHERE `+analysisFilter, args...).Scan(
		&analysis.Summary.EntryCount, &analysis.Summary.ItemCount, &analysis.Summary.StepCount, &analysis.Summary.PerformerCount,
	); err != nil {
		return Analysis{}, err
	}

	unitRows, err := r.pool.Query(ctx, `
		SELECT unit_of_measure, SUM(quantity)::DOUBLE PRECISION
		FROM operation_work_logs WHERE `+analysisFilter+`
		GROUP BY unit_of_measure ORDER BY unit_of_measure
	`, args...)
	if err != nil {
		return Analysis{}, err
	}
	for unitRows.Next() {
		var total UnitTotal
		if err := unitRows.Scan(&total.UnitOfMeasure, &total.Quantity); err != nil {
			unitRows.Close()
			return Analysis{}, err
		}
		analysis.Units = append(analysis.Units, total)
	}
	if err := unitRows.Err(); err != nil {
		unitRows.Close()
		return Analysis{}, err
	}
	unitRows.Close()

	dailyRows, err := r.pool.Query(ctx, `
		SELECT work_date::TEXT, unit_of_measure, COUNT(*), SUM(quantity)::DOUBLE PRECISION
		FROM operation_work_logs WHERE `+analysisFilter+`
		GROUP BY work_date, unit_of_measure ORDER BY work_date, unit_of_measure
	`, args...)
	if err != nil {
		return Analysis{}, err
	}
	for dailyRows.Next() {
		var total DailyTotal
		if err := dailyRows.Scan(&total.WorkDate, &total.UnitOfMeasure, &total.EntryCount, &total.Quantity); err != nil {
			dailyRows.Close()
			return Analysis{}, err
		}
		analysis.Daily = append(analysis.Daily, total)
	}
	if err := dailyRows.Err(); err != nil {
		dailyRows.Close()
		return Analysis{}, err
	}
	dailyRows.Close()

	categoryRows, err := r.pool.Query(ctx, `
		SELECT category_name, unit_of_measure, COUNT(*), SUM(quantity)::DOUBLE PRECISION
		FROM operation_work_logs WHERE `+analysisFilter+`
		GROUP BY category_name, unit_of_measure ORDER BY COUNT(*) DESC, category_name, unit_of_measure
	`, args...)
	if err != nil {
		return Analysis{}, err
	}
	for categoryRows.Next() {
		var total CategoryTotal
		if err := categoryRows.Scan(&total.CategoryName, &total.UnitOfMeasure, &total.EntryCount, &total.Quantity); err != nil {
			categoryRows.Close()
			return Analysis{}, err
		}
		analysis.Categories = append(analysis.Categories, total)
	}
	if err := categoryRows.Err(); err != nil {
		categoryRows.Close()
		return Analysis{}, err
	}
	categoryRows.Close()

	itemRows, err := r.pool.Query(ctx, `
		SELECT item_id, item_name, category_name, unit_of_measure, COUNT(*), SUM(quantity)::DOUBLE PRECISION
		FROM operation_work_logs WHERE `+analysisFilter+`
		GROUP BY item_id, item_name, category_name, unit_of_measure
		ORDER BY COUNT(*) DESC, item_name, unit_of_measure
	`, args...)
	if err != nil {
		return Analysis{}, err
	}
	for itemRows.Next() {
		var total ItemTotal
		if err := itemRows.Scan(&total.ItemID, &total.ItemName, &total.CategoryName, &total.UnitOfMeasure, &total.EntryCount, &total.Quantity); err != nil {
			itemRows.Close()
			return Analysis{}, err
		}
		analysis.Items = append(analysis.Items, total)
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return Analysis{}, err
	}
	itemRows.Close()

	stepRows, err := r.pool.Query(ctx, `
		SELECT item_name, step_title, unit_of_measure, COUNT(*), SUM(quantity)::DOUBLE PRECISION
		FROM operation_work_logs WHERE `+analysisFilter+`
		GROUP BY item_name, step_title, unit_of_measure
		ORDER BY COUNT(*) DESC, item_name, step_title, unit_of_measure
	`, args...)
	if err != nil {
		return Analysis{}, err
	}
	for stepRows.Next() {
		var total StepTotal
		if err := stepRows.Scan(&total.ItemName, &total.StepTitle, &total.UnitOfMeasure, &total.EntryCount, &total.Quantity); err != nil {
			stepRows.Close()
			return Analysis{}, err
		}
		analysis.Steps = append(analysis.Steps, total)
	}
	if err := stepRows.Err(); err != nil {
		stepRows.Close()
		return Analysis{}, err
	}
	stepRows.Close()

	performerRows, err := r.pool.Query(ctx, `
		SELECT performed_by, performer.name, log.unit_of_measure, COUNT(*), SUM(log.quantity)::DOUBLE PRECISION
		FROM operation_work_logs log
		JOIN users performer ON performer.company_id = log.company_id AND performer.id = log.performed_by
		WHERE log.`+analysisFilter+`
		GROUP BY performed_by, performer.name, log.unit_of_measure
		ORDER BY COUNT(*) DESC, performer.name, log.unit_of_measure
	`, args...)
	if err != nil {
		return Analysis{}, err
	}
	for performerRows.Next() {
		var total PerformerTotal
		if err := performerRows.Scan(&total.PerformerID, &total.PerformerName, &total.UnitOfMeasure, &total.EntryCount, &total.Quantity); err != nil {
			performerRows.Close()
			return Analysis{}, err
		}
		analysis.Performers = append(analysis.Performers, total)
	}
	if err := performerRows.Err(); err != nil {
		performerRows.Close()
		return Analysis{}, err
	}
	performerRows.Close()

	return analysis, nil
}

func (r *PostgresRepository) GetEntry(ctx context.Context, companyID, id int64) (Entry, error) {
	entry, err := scanEntry(r.pool.QueryRow(ctx, `
		SELECT log.id, log.warehouse_id, warehouse.name, log.work_date::TEXT,
		       log.item_id, log.item_name, log.category_name, log.step_id, log.step_title, log.unit_of_measure, log.quantity,
		       log.performed_by, performer.name, log.recorded_by, log.created_at::TEXT
		FROM operation_work_logs log
		JOIN warehouses warehouse ON warehouse.company_id = log.company_id AND warehouse.id = log.warehouse_id
		JOIN users performer ON performer.company_id = log.company_id AND performer.id = log.performed_by
		WHERE log.company_id = $1 AND log.id = $2
	`, companyID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	return entry, err
}

func (r *PostgresRepository) CreateEntry(ctx context.Context, companyID, recorderID int64, input Input) (Entry, error) {
	var entry Entry
	err := r.pool.QueryRow(ctx, `
		INSERT INTO operation_work_logs (
			company_id, warehouse_id, work_date, item_id, item_name, category_name,
			step_id, step_title, unit_of_measure, quantity, performed_by, recorded_by
		)
		SELECT $1, $2, $3::DATE, item.id, item.name, category.name,
		       step.id, step.title, item.unit_of_measure, $7, performer.id, $8
		FROM items item
		JOIN item_categories category ON category.company_id = item.company_id AND category.id = item.category_id
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
		          work_date::TEXT, item_id, item_name, category_name, step_id, step_title, unit_of_measure, quantity,
		          performed_by, (SELECT name FROM users WHERE company_id = $1 AND id = $6),
		          recorded_by, created_at::TEXT
	`, companyID, input.WarehouseID, input.WorkDate, input.ItemID, input.StepID,
		input.PerformedBy, input.Quantity, recorderID).Scan(
		&entry.ID, &entry.WarehouseID, &entry.WarehouseName, &entry.WorkDate,
		&entry.ItemID, &entry.ItemName, &entry.CategoryName, &entry.StepID, &entry.StepTitle, &entry.UnitOfMeasure, &entry.Quantity,
		&entry.PerformedBy, &entry.PerformerName, &entry.RecordedBy, &entry.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrInvalidInput
	}
	return entry, err
}

func (r *PostgresRepository) UpdateEntry(ctx context.Context, companyID, id int64, input Input) (Entry, error) {
	entry, err := scanEntry(r.pool.QueryRow(ctx, `
		UPDATE operation_work_logs log
		SET work_date = $3::DATE,
		    item_id = item.id,
		    item_name = item.name,
		    category_name = category.name,
		    step_id = step.id,
		    step_title = step.title,
		    unit_of_measure = item.unit_of_measure,
		    quantity = $8,
		    performed_by = performer.id
		FROM items item
		JOIN item_categories category ON category.company_id = item.company_id AND category.id = item.category_id
		JOIN item_steps step ON step.company_id = item.company_id AND step.item_id = item.id
		JOIN warehouses warehouse ON warehouse.company_id = item.company_id AND warehouse.id = $4
		JOIN users performer ON performer.company_id = item.company_id AND performer.id = $7 AND performer.is_active
		WHERE log.company_id = $1 AND log.id = $2 AND log.warehouse_id = $4
		  AND item.company_id = $1 AND item.id = $5 AND item.is_active AND step.id = $6
		  AND EXISTS (
			SELECT 1
			FROM user_role_assignments assignment
			JOIN roles role ON role.company_id = assignment.company_id AND role.id = assignment.role_id
			WHERE assignment.company_id = item.company_id
			  AND assignment.user_id = performer.id
			  AND assignment.warehouse_id = warehouse.id
			  AND role.slug = 'worker'
		  )
		RETURNING log.id, log.warehouse_id,
		          (SELECT name FROM warehouses WHERE company_id = $1 AND id = $4),
		          log.work_date::TEXT, log.item_id, log.item_name, log.category_name, log.step_id, log.step_title,
		          log.unit_of_measure, log.quantity, log.performed_by,
		          (SELECT name FROM users WHERE company_id = $1 AND id = $7),
		          log.recorded_by, log.created_at::TEXT
	`, companyID, id, input.WorkDate, input.WarehouseID, input.ItemID, input.StepID,
		input.PerformedBy, input.Quantity))
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrInvalidInput
	}
	return entry, err
}

func scanEntry(row pgx.Row) (Entry, error) {
	var entry Entry
	err := row.Scan(&entry.ID, &entry.WarehouseID, &entry.WarehouseName, &entry.WorkDate,
		&entry.ItemID, &entry.ItemName, &entry.CategoryName, &entry.StepID, &entry.StepTitle, &entry.UnitOfMeasure, &entry.Quantity,
		&entry.PerformedBy, &entry.PerformerName, &entry.RecordedBy, &entry.CreatedAt)
	return entry, err
}
