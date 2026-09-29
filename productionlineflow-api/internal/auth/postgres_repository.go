package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) FindUserByLogin(ctx context.Context, companySlug, email string) (User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
        SELECT u.id, u.company_id, c.slug, u.name, u.email, u.password_hash, u.is_active, u.perm_version
        FROM users u
        JOIN companies c ON c.id = u.company_id
		WHERE c.slug = $1 AND u.email = $2 AND c.status = 'active'
    `, companySlug, email))
}

func (r *PostgresRepository) FindUserByID(ctx context.Context, userID, companyID int64) (User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
        SELECT u.id, u.company_id, c.slug, u.name, u.email, u.password_hash, u.is_active, u.perm_version
        FROM users u
        JOIN companies c ON c.id = u.company_id
		WHERE u.id = $1 AND u.company_id = $2 AND c.status = 'active'
    `, userID, companyID))
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.CompanyID, &user.CompanySlug, &user.Name, &user.Email, &user.PasswordHash, &user.IsActive, &user.PermVersion)
	return user, err
}

func (r *PostgresRepository) ListPermissions(ctx context.Context, userID, companyID int64) ([]Permission, error) {
	rows, err := r.pool.Query(ctx, `
        SELECT p.key, ura.warehouse_id, roles.slug, roles.scope
        FROM user_role_assignments ura
        JOIN roles ON roles.id = ura.role_id AND roles.company_id = ura.company_id
        JOIN role_permissions rp ON rp.role_id = roles.id
        JOIN permissions p ON p.id = rp.permission_id
        WHERE ura.user_id = $1 AND ura.company_id = $2
        ORDER BY p.key, ura.warehouse_id
    `, userID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var permission Permission
		if err := rows.Scan(&permission.Key, &permission.WarehouseID, &permission.RoleSlug, &permission.RoleScope); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func (r *PostgresRepository) CreateRefreshToken(ctx context.Context, userID, companyID int64, hash string, expiresAt time.Time, userAgent string) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO refresh_tokens (company_id, user_id, token_hash, expires_at, user_agent)
        VALUES ($1, $2, $3, $4, $5)
    `, companyID, userID, hash, expiresAt, userAgent)
	return err
}

func (r *PostgresRepository) RotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time, userAgent string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var (
		tokenID     int64
		userID      int64
		companyID   int64
		tokenExpiry time.Time
		revokedAt   *time.Time
		replacedBy  *int64
	)
	row := tx.QueryRow(ctx, `
        SELECT rt.id, rt.user_id, rt.company_id, rt.expires_at, rt.revoked_at, rt.replaced_by
        FROM refresh_tokens rt
        WHERE rt.token_hash = $1
        FOR UPDATE
    `, oldHash)
	if err := row.Scan(&tokenID, &userID, &companyID, &tokenExpiry, &revokedAt, &replacedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrRefreshInvalid
		}
		return User{}, err
	}
	if revokedAt != nil {
		if replacedBy != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE refresh_tokens
				SET revoked_at = COALESCE(revoked_at, now())
				WHERE user_id = $1 AND company_id = $2 AND revoked_at IS NULL
			`, userID, companyID); err != nil {
				return User{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return User{}, err
			}
			return User{}, ErrRefreshReused
		}
		return User{}, ErrRefreshInvalid
	}
	if time.Now().After(tokenExpiry) {
		return User{}, ErrRefreshInvalid
	}

	var newTokenID int64
	if err := tx.QueryRow(ctx, `
        INSERT INTO refresh_tokens (company_id, user_id, token_hash, expires_at, user_agent)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `, companyID, userID, newHash, expiresAt, userAgent).Scan(&newTokenID); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $1 WHERE id = $2`, newTokenID, tokenID); err != nil {
		return User{}, err
	}
	user, err := scanUser(tx.QueryRow(ctx, `
        SELECT u.id, u.company_id, c.slug, u.name, u.email, u.password_hash, u.is_active, u.perm_version
        FROM users u
        JOIN companies c ON c.id = u.company_id
		WHERE u.id = $1 AND u.company_id = $2 AND c.status = 'active'
    `, userID, companyID))
	if err != nil {
		return User{}, err
	}
	if !user.IsActive {
		return User{}, ErrRefreshInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *PostgresRepository) RevokeRefreshToken(ctx context.Context, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE token_hash = $1`, hash)
	return err
}

func (r *PostgresRepository) FindPlatformUserByLogin(ctx context.Context, email string) (PlatformUser, error) {
	return scanPlatformUser(r.pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash, is_active, perm_version
		FROM platform_users WHERE email = $1
	`, email))
}

func (r *PostgresRepository) FindPlatformUserByID(ctx context.Context, userID int64) (PlatformUser, error) {
	return scanPlatformUser(r.pool.QueryRow(ctx, `
		SELECT id, name, email, password_hash, is_active, perm_version
		FROM platform_users WHERE id = $1
	`, userID))
}

func scanPlatformUser(row pgx.Row) (PlatformUser, error) {
	var user PlatformUser
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.IsActive, &user.PermVersion)
	return user, err
}

func (r *PostgresRepository) ListPlatformPermissions(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT p.key
		FROM platform_user_role_assignments ura
		JOIN platform_role_permissions rp ON rp.platform_role_id = ura.platform_role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ura.platform_user_id = $1
		ORDER BY p.key
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var permissions []string
	for rows.Next() {
		var permission string
		if err := rows.Scan(&permission); err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func (r *PostgresRepository) CreatePlatformRefreshToken(ctx context.Context, userID int64, hash string, expiresAt time.Time, userAgent string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO platform_refresh_tokens (platform_user_id, token_hash, expires_at, user_agent)
		VALUES ($1, $2, $3, $4)
	`, userID, hash, expiresAt, userAgent)
	return err
}

func (r *PostgresRepository) RotatePlatformRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time, userAgent string) (PlatformUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return PlatformUser{}, err
	}
	defer tx.Rollback(ctx)

	var tokenID, userID int64
	var tokenExpiry time.Time
	var revokedAt *time.Time
	var replacedBy *int64
	if err := tx.QueryRow(ctx, `
		SELECT id, platform_user_id, expires_at, revoked_at, replaced_by
		FROM platform_refresh_tokens WHERE token_hash = $1 FOR UPDATE
	`, oldHash).Scan(&tokenID, &userID, &tokenExpiry, &revokedAt, &replacedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlatformUser{}, ErrPlatformRefreshInvalid
		}
		return PlatformUser{}, err
	}
	if revokedAt != nil {
		if replacedBy != nil {
			if _, err := tx.Exec(ctx, `UPDATE platform_refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE platform_user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
				return PlatformUser{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return PlatformUser{}, err
			}
			return PlatformUser{}, ErrPlatformRefreshReused
		}
		return PlatformUser{}, ErrPlatformRefreshInvalid
	}
	if time.Now().After(tokenExpiry) {
		return PlatformUser{}, ErrPlatformRefreshInvalid
	}

	var newTokenID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO platform_refresh_tokens (platform_user_id, token_hash, expires_at, user_agent)
		VALUES ($1, $2, $3, $4) RETURNING id
	`, userID, newHash, expiresAt, userAgent).Scan(&newTokenID); err != nil {
		return PlatformUser{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE platform_refresh_tokens SET revoked_at = now(), replaced_by = $1 WHERE id = $2`, newTokenID, tokenID); err != nil {
		return PlatformUser{}, err
	}
	user, err := scanPlatformUser(tx.QueryRow(ctx, `
		SELECT id, name, email, password_hash, is_active, perm_version FROM platform_users WHERE id = $1
	`, userID))
	if err != nil {
		return PlatformUser{}, err
	}
	if !user.IsActive {
		return PlatformUser{}, ErrPlatformRefreshInvalid
	}
	if err := tx.Commit(ctx); err != nil {
		return PlatformUser{}, err
	}
	return user, nil
}

func (r *PostgresRepository) RevokePlatformRefreshToken(ctx context.Context, hash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE platform_refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE token_hash = $1`, hash)
	return err
}
