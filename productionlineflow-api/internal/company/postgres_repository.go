package company

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

func (r *PostgresRepository) CreateCompany(ctx context.Context, input Input, passwordHash string) (Result, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	var result Result
	if err := tx.QueryRow(ctx, `INSERT INTO companies (slug, name) VALUES ($1, $2) RETURNING id, slug, name`, input.Slug, input.Name).Scan(&result.ID, &result.Slug, &result.Name); err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "companies_slug_key" {
			return Result{}, ErrDuplicateSlug
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, err
		}
		return Result{}, err
	}
	roleIDs := map[string]int64{}
	roles := []struct{ slug, name, scope string }{
		{"super_admin", "Super Admin", "company"}, {"admin", "Admin", "company"}, {"manager", "Manager", "warehouse"}, {"worker", "Worker", "warehouse"},
	}
	for _, role := range roles {
		var id int64
		if err := tx.QueryRow(ctx, `INSERT INTO roles (company_id, slug, name, scope, is_system) VALUES ($1, $2, $3, $4, true) RETURNING id`, result.ID, role.slug, role.name, role.scope).Scan(&id); err != nil {
			return Result{}, err
		}
		roleIDs[role.slug] = id
	}
	defaults := map[string][]string{
		"super_admin": {"admins.manage", "roles.manage", "users.create", "warehouse_types.manage", "warehouses.manage", "warehouse.members.manage", "warehouse.view", "workers.tasks.execute", "items.view", "items.manage", "items.categories.view", "items.categories.manage", "operations.logs.view", "operations.logs.create", "operations.logs.date_override"},
		"admin":       {"users.create", "warehouse_types.manage", "warehouses.manage", "warehouse.members.manage", "warehouse.view", "workers.tasks.execute", "items.view", "items.manage", "items.categories.view", "items.categories.manage", "operations.logs.view", "operations.logs.create", "operations.logs.date_override"},
		"manager":     {"warehouse.members.manage", "warehouse.view", "workers.tasks.execute", "operations.logs.view", "operations.logs.create"},
		"worker":      {"warehouse.view", "workers.tasks.execute"},
	}
	for slug, permissions := range defaults {
		for _, key := range permissions {
			if _, err := tx.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) SELECT $1, id FROM permissions WHERE key = $2`, roleIDs[slug], key); err != nil {
				return Result{}, err
			}
		}
	}
	if err := tx.QueryRow(ctx, `INSERT INTO users (company_id, name, email, password_hash) VALUES ($1, $2, $3, $4) RETURNING id, name, email`, result.ID, input.SuperAdminName, input.SuperAdminEmail, passwordHash).Scan(&result.SuperAdminID, &result.SuperAdminName, &result.SuperAdminEmail); err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "users_company_email" {
			return Result{}, ErrDuplicateAdminEmail
		}
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_role_assignments (company_id, user_id, role_id) VALUES ($1, $2, $3)`, result.ID, result.SuperAdminID, roleIDs["super_admin"]); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (r *PostgresRepository) ListCompanies(ctx context.Context) ([]Company, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, slug, name, status, suspended_at, activated_at FROM companies ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	companies := make([]Company, 0)
	for rows.Next() {
		var item Company
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Status, &item.SuspendedAt, &item.ActivatedAt); err != nil {
			return nil, err
		}
		companies = append(companies, item)
	}
	return companies, rows.Err()
}

func (r *PostgresRepository) UpdateCompany(ctx context.Context, id int64, input UpdateInput) (Company, error) {
	var item Company
	err := r.pool.QueryRow(ctx, `
		UPDATE companies SET name = $2, slug = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, slug, name, status, suspended_at, activated_at
	`, id, input.Name, input.Slug).Scan(&item.ID, &item.Slug, &item.Name, &item.Status, &item.SuspendedAt, &item.ActivatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return Company{}, ErrDuplicateSlug
	}
	return item, err
}

func (r *PostgresRepository) SuspendCompany(ctx context.Context, id int64) (Company, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Company{}, err
	}
	defer tx.Rollback(ctx)
	var item Company
	err = tx.QueryRow(ctx, `
		UPDATE companies SET status = 'suspended', suspended_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, slug, name, status, suspended_at, activated_at
	`, id).Scan(&item.ID, &item.Slug, &item.Name, &item.Status, &item.SuspendedAt, &item.ActivatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, ErrNotFound
	}
	if err != nil {
		return Company{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE tenant_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE company_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return Company{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE company_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return Company{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Company{}, err
	}
	return item, nil
}

func (r *PostgresRepository) ReactivateCompany(ctx context.Context, id int64) (Company, error) {
	var item Company
	err := r.pool.QueryRow(ctx, `
		UPDATE companies SET status = 'active', activated_at = now(), updated_at = now()
		WHERE id = $1
		RETURNING id, slug, name, status, suspended_at, activated_at
	`, id).Scan(&item.ID, &item.Slug, &item.Name, &item.Status, &item.SuspendedAt, &item.ActivatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, ErrNotFound
	}
	return item, err
}
