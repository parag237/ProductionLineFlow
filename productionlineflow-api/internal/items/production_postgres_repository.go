package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductionPostgresRepository struct{ pool *pgxpool.Pool }

func NewProductionPostgresRepository(pool *pgxpool.Pool) *ProductionPostgresRepository {
	return &ProductionPostgresRepository{pool: pool}
}

func (r *ProductionPostgresRepository) ListRuns(ctx context.Context, companyID int64) ([]ProductionRun, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, item_id, item_name, tracking_mode, quantity, status, created_by, started_at::text, completed_at::text FROM production_runs WHERE company_id = $1 ORDER BY created_at DESC, id DESC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := make([]ProductionRun, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := loadRunsDetails(ctx, r.pool, companyID, runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func (r *ProductionPostgresRepository) GetRun(ctx context.Context, companyID, id int64) (ProductionRun, error) {
	run, err := scanRun(r.pool.QueryRow(ctx, `SELECT id, item_id, item_name, tracking_mode, quantity, status, created_by, started_at::text, completed_at::text FROM production_runs WHERE company_id = $1 AND id = $2`, companyID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductionRun{}, ErrNotFound
	}
	if err != nil {
		return ProductionRun{}, err
	}
	return loadSingleRun(ctx, r.pool, companyID, run)
}

func (r *ProductionPostgresRepository) CreateRun(ctx context.Context, companyID, userID int64, input RunInput) (ProductionRun, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ProductionRun{}, err
	}
	defer tx.Rollback(ctx)

	var itemName string
	var stepCount int
	err = tx.QueryRow(ctx, `SELECT name FROM items WHERE company_id = $1 AND id = $2 AND is_active FOR SHARE`, companyID, input.ItemID).Scan(&itemName)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductionRun{}, ErrNotFound
	}
	if err != nil {
		return ProductionRun{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM item_steps WHERE company_id = $1 AND item_id = $2`, companyID, input.ItemID).Scan(&stepCount); err != nil {
		return ProductionRun{}, err
	}
	if stepCount == 0 {
		return ProductionRun{}, ErrInvalidProgress
	}

	var runID int64
	err = tx.QueryRow(ctx, `INSERT INTO production_runs(company_id, item_id, item_name, tracking_mode, quantity, created_by) VALUES($1, $2, $3, $4, $5, $6) RETURNING id`, companyID, input.ItemID, itemName, input.TrackingMode, input.Quantity, userID).Scan(&runID)
	if err != nil {
		return ProductionRun{}, mapProductionConflict(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO production_run_steps(company_id, run_id, position, title, instructions) SELECT company_id, $3, position, title, instructions FROM item_steps WHERE company_id = $1 AND item_id = $2 ORDER BY position`, companyID, input.ItemID, runID); err != nil {
		return ProductionRun{}, err
	}
	for _, serialNumber := range input.SerialNumbers {
		if _, err := tx.Exec(ctx, `INSERT INTO production_units(company_id, run_id, serial_number) VALUES($1, $2, $3)`, companyID, runID, serialNumber); err != nil {
			return ProductionRun{}, mapProductionConflict(err)
		}
	}
	run, err := getRun(ctx, tx, companyID, runID)
	if err != nil {
		return ProductionRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProductionRun{}, mapProductionConflict(err)
	}
	return run, nil
}

func (r *ProductionPostgresRepository) CompleteStep(ctx context.Context, companyID, userID, runID, stepID int64, input StepCompletionInput) (ProductionRun, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ProductionRun{}, err
	}
	defer tx.Rollback(ctx)

	var trackingMode, status string
	err = tx.QueryRow(ctx, `SELECT tracking_mode, status FROM production_runs WHERE company_id = $1 AND id = $2 FOR UPDATE`, companyID, runID).Scan(&trackingMode, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductionRun{}, ErrNotFound
	}
	if err != nil {
		return ProductionRun{}, err
	}
	if status != "in_progress" {
		return ProductionRun{}, ErrInvalidProgress
	}

	var position int
	if err := tx.QueryRow(ctx, `SELECT position FROM production_run_steps WHERE company_id = $1 AND run_id = $2 AND id = $3`, companyID, runID, stepID).Scan(&position); errors.Is(err, pgx.ErrNoRows) {
		return ProductionRun{}, ErrNotFound
	} else if err != nil {
		return ProductionRun{}, err
	}

	var unitValue any
	if trackingMode == "unit" {
		if input.UnitID == nil {
			return ProductionRun{}, ErrInvalidProgress
		}
		unitValue = *input.UnitID
		var unitStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM production_units WHERE company_id = $1 AND run_id = $2 AND id = $3 FOR UPDATE`, companyID, runID, *input.UnitID).Scan(&unitStatus); errors.Is(err, pgx.ErrNoRows) {
			return ProductionRun{}, ErrNotFound
		} else if err != nil {
			return ProductionRun{}, err
		}
		if unitStatus != "in_progress" {
			return ProductionRun{}, ErrInvalidProgress
		}
	} else if input.UnitID != nil {
		return ProductionRun{}, ErrInvalidProgress
	}

	var previousIncomplete bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM production_run_steps previous
		WHERE previous.company_id = $1 AND previous.run_id = $2 AND previous.position < $3
		AND NOT EXISTS (
			SELECT 1 FROM production_step_events event
			WHERE event.company_id = $1 AND event.run_id = $2 AND event.run_step_id = previous.id
			AND event.event = 'completed' AND (($4::bigint IS NULL AND event.unit_id IS NULL) OR event.unit_id = $4)
		)
	)`, companyID, runID, position, unitValue).Scan(&previousIncomplete)
	if err != nil {
		return ProductionRun{}, err
	}
	if previousIncomplete {
		return ProductionRun{}, ErrInvalidProgress
	}
	if _, err := tx.Exec(ctx, `INSERT INTO production_step_events(company_id, run_id, run_step_id, unit_id, event, actor_id, note) VALUES($1, $2, $3, $4, 'completed', $5, $6)`, companyID, runID, stepID, unitValue, userID, input.Note); err != nil {
		return ProductionRun{}, mapProductionConflict(err)
	}

	if trackingMode == "unit" {
		if _, err := tx.Exec(ctx, `UPDATE production_units unit SET status = 'completed' WHERE unit.company_id = $1 AND unit.run_id = $2 AND unit.id = $3 AND NOT EXISTS (
			SELECT 1 FROM production_run_steps step WHERE step.company_id = $1 AND step.run_id = $2 AND NOT EXISTS (
				SELECT 1 FROM production_step_events event WHERE event.company_id = $1 AND event.run_id = $2 AND event.run_step_id = step.id AND event.unit_id = unit.id AND event.event = 'completed'
			)
		)`, companyID, runID, *input.UnitID); err != nil {
			return ProductionRun{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE production_runs SET status = 'completed', completed_at = now() WHERE company_id = $1 AND id = $2 AND NOT EXISTS (SELECT 1 FROM production_units WHERE company_id = $1 AND run_id = $2 AND status <> 'completed')`, companyID, runID); err != nil {
			return ProductionRun{}, err
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE production_runs SET status = 'completed', completed_at = now() WHERE company_id = $1 AND id = $2 AND NOT EXISTS (
			SELECT 1 FROM production_run_steps step WHERE step.company_id = $1 AND step.run_id = $2 AND NOT EXISTS (
				SELECT 1 FROM production_step_events event WHERE event.company_id = $1 AND event.run_id = $2 AND event.run_step_id = step.id AND event.unit_id IS NULL AND event.event = 'completed'
			)
		)`, companyID, runID); err != nil {
			return ProductionRun{}, err
		}
	}

	run, err := getRun(ctx, tx, companyID, runID)
	if err != nil {
		return ProductionRun{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ProductionRun{}, mapProductionConflict(err)
	}
	return run, nil
}

func scanRun(row pgx.Row) (ProductionRun, error) {
	var run ProductionRun
	var completedAt *string
	err := row.Scan(&run.ID, &run.ItemID, &run.ItemName, &run.TrackingMode, &run.Quantity, &run.Status, &run.CreatedBy, &run.StartedAt, &completedAt)
	run.CompletedAt = completedAt
	run.Steps = []RunStep{}
	run.Units = []ProductionUnit{}
	run.Events = []StepEvent{}
	return run, err
}

func getRun(ctx context.Context, db queryer, companyID, runID int64) (ProductionRun, error) {
	run, err := scanRun(db.QueryRow(ctx, `SELECT id, item_id, item_name, tracking_mode, quantity, status, created_by, started_at::text, completed_at::text FROM production_runs WHERE company_id = $1 AND id = $2`, companyID, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ProductionRun{}, ErrNotFound
	}
	if err != nil {
		return ProductionRun{}, err
	}
	return loadSingleRun(ctx, db, companyID, run)
}

func loadSingleRun(ctx context.Context, db queryer, companyID int64, run ProductionRun) (ProductionRun, error) {
	if err := loadStepsForRun(ctx, db, companyID, &run); err != nil {
		return ProductionRun{}, err
	}
	if err := loadUnitsForRun(ctx, db, companyID, &run); err != nil {
		return ProductionRun{}, err
	}
	if err := loadEventsForRun(ctx, db, companyID, &run); err != nil {
		return ProductionRun{}, err
	}
	return run, nil
}

func loadRunsDetails(ctx context.Context, db queryer, companyID int64, runs []ProductionRun) error {
	if len(runs) == 0 {
		return nil
	}
	indexes := make(map[int64]int, len(runs))
	ids := make([]int64, len(runs))
	for index := range runs {
		indexes[runs[index].ID] = index
		ids[index] = runs[index].ID
	}

	stepRows, err := db.Query(ctx, `SELECT run_id, id, position, title, instructions FROM production_run_steps WHERE company_id = $1 AND run_id = ANY($2) ORDER BY run_id, position`, companyID, ids)
	if err != nil {
		return err
	}
	for stepRows.Next() {
		var runID int64
		var step RunStep
		if err := stepRows.Scan(&runID, &step.ID, &step.Position, &step.Title, &step.Instructions); err != nil {
			stepRows.Close()
			return err
		}
		runs[indexes[runID]].Steps = append(runs[indexes[runID]].Steps, step)
	}
	if err := stepRows.Err(); err != nil {
		stepRows.Close()
		return err
	}
	stepRows.Close()

	unitRows, err := db.Query(ctx, `SELECT run_id, id, serial_number, status FROM production_units WHERE company_id = $1 AND run_id = ANY($2) ORDER BY run_id, serial_number`, companyID, ids)
	if err != nil {
		return err
	}
	for unitRows.Next() {
		var runID int64
		var unit ProductionUnit
		if err := unitRows.Scan(&runID, &unit.ID, &unit.SerialNumber, &unit.Status); err != nil {
			unitRows.Close()
			return err
		}
		runs[indexes[runID]].Units = append(runs[indexes[runID]].Units, unit)
	}
	if err := unitRows.Err(); err != nil {
		unitRows.Close()
		return err
	}
	unitRows.Close()

	eventRows, err := db.Query(ctx, `SELECT run_id, id, run_step_id, unit_id, event, actor_id, note, created_at::text FROM production_step_events WHERE company_id = $1 AND run_id = ANY($2) ORDER BY run_id, created_at, id`, companyID, ids)
	if err != nil {
		return err
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var runID int64
		var event StepEvent
		if err := eventRows.Scan(&runID, &event.ID, &event.RunStepID, &event.UnitID, &event.Event, &event.ActorID, &event.Note, &event.CreatedAt); err != nil {
			return err
		}
		runs[indexes[runID]].Events = append(runs[indexes[runID]].Events, event)
	}
	return eventRows.Err()
}

func loadStepsForRun(ctx context.Context, db queryer, companyID int64, run *ProductionRun) error {
	rows, err := db.Query(ctx, `SELECT id, position, title, instructions FROM production_run_steps WHERE company_id = $1 AND run_id = $2 ORDER BY position`, companyID, run.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var step RunStep
		if err := rows.Scan(&step.ID, &step.Position, &step.Title, &step.Instructions); err != nil {
			return err
		}
		run.Steps = append(run.Steps, step)
	}
	return rows.Err()
}

func loadUnitsForRun(ctx context.Context, db queryer, companyID int64, run *ProductionRun) error {
	rows, err := db.Query(ctx, `SELECT id, serial_number, status FROM production_units WHERE company_id = $1 AND run_id = $2 ORDER BY serial_number`, companyID, run.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var unit ProductionUnit
		if err := rows.Scan(&unit.ID, &unit.SerialNumber, &unit.Status); err != nil {
			return err
		}
		run.Units = append(run.Units, unit)
	}
	return rows.Err()
}

func loadEventsForRun(ctx context.Context, db queryer, companyID int64, run *ProductionRun) error {
	rows, err := db.Query(ctx, `SELECT id, run_step_id, unit_id, event, actor_id, note, created_at::text FROM production_step_events WHERE company_id = $1 AND run_id = $2 ORDER BY created_at, id`, companyID, run.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var event StepEvent
		if err := rows.Scan(&event.ID, &event.RunStepID, &event.UnitID, &event.Event, &event.ActorID, &event.Note, &event.CreatedAt); err != nil {
			return err
		}
		run.Events = append(run.Events, event)
	}
	return rows.Err()
}

func mapProductionConflict(err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505":
			return fmt.Errorf("%w: %v", ErrConflict, err)
		case "23503":
			return fmt.Errorf("%w: %v", ErrInvalidProgress, err)
		}
	}
	return err
}
