package company

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

func (r *PostgresRepository) CreateCompany(ctx context.Context, input Input, passwordHash string) (Result, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)

	var result Result
	if err := tx.QueryRow(ctx, `INSERT INTO companies (slug, name) VALUES ($1, $2) RETURNING id, slug, name`, input.Slug, input.Name).Scan(&result.ID, &result.Slug, &result.Name); err != nil {
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
		"super_admin": {"admins.manage", "roles.manage", "users.create", "warehouse_types.manage", "warehouses.manage", "warehouse.members.manage", "warehouse.view", "workers.tasks.execute"},
		"admin":       {"users.create", "warehouse_types.manage", "warehouses.manage", "warehouse.members.manage", "warehouse.view", "workers.tasks.execute"},
		"manager":     {"warehouse.members.manage", "warehouse.view", "workers.tasks.execute"},
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
