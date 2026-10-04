package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

type queryer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

const itemSelect = `SELECT id, name, COALESCE(sku, ''), description, unit_of_measure, is_active, created_at::text, updated_at::text FROM items WHERE company_id = $1`

func scanItem(row pgx.Row) (Item, error) {
	var item Item
	err := row.Scan(&item.ID, &item.Name, &item.SKU, &item.Description, &item.UnitOfMeasure, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
	item.Steps = []Step{}
	return item, err
}

func (r *PostgresRepository) ListItems(ctx context.Context, companyID int64) ([]Item, error) {
	rows, err := r.pool.Query(ctx, itemSelect+` ORDER BY name, id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	ids := make([]int64, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	steps, err := loadSteps(ctx, r.pool, companyID, ids)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Steps = steps[items[index].ID]
	}
	return items, nil
}

func (r *PostgresRepository) GetItem(ctx context.Context, companyID, id int64) (Item, error) {
	return getItem(ctx, r.pool, companyID, id)
}

func (r *PostgresRepository) CreateItem(ctx context.Context, companyID int64, input Input) (Item, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback(ctx)

	var sku any
	if input.SKU != "" {
		sku = input.SKU
	}
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO items(company_id, name, sku, description, unit_of_measure) VALUES($1, $2, $3, $4, $5) RETURNING id`, companyID, input.Name, sku, input.Description, input.UnitOfMeasure).Scan(&id)
	if err != nil {
		return Item{}, mapItemConflict(err)
	}
	if err := insertSteps(ctx, tx, companyID, id, input.Steps); err != nil {
		return Item{}, err
	}
	item, err := getItem(ctx, tx, companyID, id)
	if err != nil {
		return Item{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Item{}, mapItemConflict(err)
	}
	return item, nil
}

func (r *PostgresRepository) UpdateItem(ctx context.Context, companyID, id int64, input Input) (Item, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback(ctx)

	var sku any
	if input.SKU != "" {
		sku = input.SKU
	}
	result, err := tx.Exec(ctx, `UPDATE items SET name = $3, sku = $4, description = $5, unit_of_measure = $6, updated_at = now() WHERE company_id = $1 AND id = $2`, companyID, id, input.Name, sku, input.Description, input.UnitOfMeasure)
	if err != nil {
		return Item{}, mapItemConflict(err)
	}
	if result.RowsAffected() == 0 {
		return Item{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM item_steps WHERE company_id = $1 AND item_id = $2`, companyID, id); err != nil {
		return Item{}, err
	}
	if err := insertSteps(ctx, tx, companyID, id, input.Steps); err != nil {
		return Item{}, err
	}
	item, err := getItem(ctx, tx, companyID, id)
	if err != nil {
		return Item{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Item{}, mapItemConflict(err)
	}
	return item, nil
}

func (r *PostgresRepository) ArchiveItem(ctx context.Context, companyID, id int64) error {
	result, err := r.pool.Exec(ctx, `UPDATE items SET is_active = false, updated_at = now() WHERE company_id = $1 AND id = $2`, companyID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func getItem(ctx context.Context, db queryer, companyID, id int64) (Item, error) {
	item, err := scanItem(db.QueryRow(ctx, itemSelect+` AND id = $2`, companyID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	steps, err := loadSteps(ctx, db, companyID, []int64{id})
	if err != nil {
		return Item{}, err
	}
	item.Steps = steps[id]
	return item, nil
}

func loadSteps(ctx context.Context, db queryer, companyID int64, itemIDs []int64) (map[int64][]Step, error) {
	result := make(map[int64][]Step, len(itemIDs))
	for _, id := range itemIDs {
		result[id] = []Step{}
	}
	if len(itemIDs) == 0 {
		return result, nil
	}
	rows, err := db.Query(ctx, `SELECT item_id, id, position, title, instructions FROM item_steps WHERE company_id = $1 AND item_id = ANY($2) ORDER BY item_id, position`, companyID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID int64
		var step Step
		if err := rows.Scan(&itemID, &step.ID, &step.Position, &step.Title, &step.Instructions); err != nil {
			return nil, err
		}
		result[itemID] = append(result[itemID], step)
	}
	return result, rows.Err()
}

func insertSteps(ctx context.Context, db queryer, companyID, itemID int64, steps []StepInput) error {
	for index, step := range steps {
		if _, err := db.Exec(ctx, `INSERT INTO item_steps(company_id, item_id, position, title, instructions) VALUES($1, $2, $3, $4, $5)`, companyID, itemID, index+1, step.Title, step.Instructions); err != nil {
			return fmt.Errorf("insert item step %d: %w", index+1, err)
		}
	}
	return nil
}

func mapItemConflict(err error) error {
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "items_company_sku" {
		return ErrConflict
	}
	return err
}
