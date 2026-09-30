package warehouse

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

const warehouseSelect = `SELECT w.id, w.company_id, w.type_id, wt.name, w.name, COALESCE(w.address, ''), w.state, w.created_at::text FROM warehouses w JOIN warehouse_types wt ON wt.id = w.type_id WHERE w.company_id = $1`

func scanWarehouse(row pgx.Row) (Warehouse, error) {
	var item Warehouse
	err := row.Scan(&item.ID, &item.CompanyID, &item.TypeID, &item.TypeName, &item.Name, &item.Address, &item.State, &item.CreatedAt)
	return item, err
}

func (r *PostgresRepository) List(ctx context.Context, companyID int64) ([]Warehouse, error) {
	rows, err := r.pool.Query(ctx, warehouseSelect+` ORDER BY w.name, w.id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Warehouse, 0)
	for rows.Next() {
		item, err := scanWarehouse(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Create(ctx context.Context, companyID int64, input Input) (Warehouse, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Warehouse{}, err
	}
	defer tx.Rollback(ctx)
	var typeID int64
	if err := tx.QueryRow(ctx, `INSERT INTO warehouse_types (company_id, name) VALUES ($1, $2) ON CONFLICT (company_id, name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, companyID, input.TypeName).Scan(&typeID); err != nil {
		return Warehouse{}, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO warehouses (company_id, type_id, name, address) VALUES ($1, $2, $3, $4) RETURNING id`, companyID, typeID, input.Name, input.Address).Scan(&id); err != nil {
		return Warehouse{}, err
	}
	item, err := scanWarehouse(tx.QueryRow(ctx, warehouseSelect+` AND w.id = $2`, companyID, id))
	if err != nil {
		return Warehouse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Warehouse{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Get(ctx context.Context, companyID, id int64) (Warehouse, error) {
	item, err := scanWarehouse(r.pool.QueryRow(ctx, warehouseSelect+` AND w.id = $2`, companyID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Warehouse{}, ErrNotFound
	}
	return item, err
}

func (r *PostgresRepository) Update(ctx context.Context, companyID, id int64, input Input) (Warehouse, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Warehouse{}, err
	}
	defer tx.Rollback(ctx)
	var typeID int64
	if err := tx.QueryRow(ctx, `INSERT INTO warehouse_types (company_id, name) VALUES ($1, $2) ON CONFLICT (company_id, name) DO UPDATE SET name = EXCLUDED.name RETURNING id`, companyID, input.TypeName).Scan(&typeID); err != nil {
		return Warehouse{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE warehouses SET name = $3, address = $4, type_id = $2 WHERE company_id = $1 AND id = $5`, companyID, typeID, input.Name, input.Address, id); err != nil {
		return Warehouse{}, err
	}
	item, err := scanWarehouse(tx.QueryRow(ctx, warehouseSelect+` AND w.id = $2`, companyID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Warehouse{}, ErrNotFound
	}
	if err != nil {
		return Warehouse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Warehouse{}, err
	}
	return item, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, companyID, id int64) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM warehouses WHERE company_id = $1 AND id = $2`, companyID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
