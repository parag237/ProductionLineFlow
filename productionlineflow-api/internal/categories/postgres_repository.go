package categories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context, companyID int64) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, created_at::text, updated_at::text FROM item_categories WHERE company_id = $1 ORDER BY lower(name), id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	categories := make([]Category, 0)
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name, &category.CreatedAt, &category.UpdatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (r *PostgresRepository) Create(ctx context.Context, companyID int64, name string) (Category, error) {
	var category Category
	err := r.pool.QueryRow(ctx, `INSERT INTO item_categories (company_id, name) VALUES ($1, $2) RETURNING id, name, created_at::text, updated_at::text`, companyID, name).Scan(&category.ID, &category.Name, &category.CreatedAt, &category.UpdatedAt)
	return category, mapConflict(err)
}

func (r *PostgresRepository) Update(ctx context.Context, companyID, id int64, name string) (Category, error) {
	var category Category
	err := r.pool.QueryRow(ctx, `UPDATE item_categories SET name = $3, updated_at = now() WHERE company_id = $1 AND id = $2 RETURNING id, name, created_at::text, updated_at::text`, companyID, id, name).Scan(&category.ID, &category.Name, &category.CreatedAt, &category.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return category, mapConflict(err)
}

func (r *PostgresRepository) Delete(ctx context.Context, companyID, id int64) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM item_categories WHERE company_id = $1 AND id = $2`, companyID, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrConflict
		}
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func mapConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
