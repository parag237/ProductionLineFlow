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

func (r *PostgresRepository) CreateRefreshToken(ctx context.Context, userID, companyID int64, sessionID, hash string, expiresAt time.Time, userAgent string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO tenant_auth_sessions(id,company_id,user_id,last_activity_at,idle_expires_at) VALUES($1,$2,$3,now(),$4)`, sessionID, companyID, userID, expiresAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO refresh_tokens(company_id,user_id,session_id,token_hash,expires_at,user_agent) VALUES($1,$2,$3,$4,$5,$6)`, companyID, userID, sessionID, hash, expiresAt, userAgent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) RotateRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time, userAgent string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var (
		tokenID        int64
		userID         int64
		companyID      int64
		tokenExpiry    time.Time
		revokedAt      *time.Time
		replacedBy     *int64
		sessionIDValue *string
	)
	row := tx.QueryRow(ctx, `
		SELECT rt.id, rt.user_id, rt.company_id, rt.expires_at, rt.revoked_at, rt.replaced_by, rt.session_id
        FROM refresh_tokens rt
        WHERE rt.token_hash = $1
        FOR UPDATE
	`, oldHash)
	if err := row.Scan(&tokenID, &userID, &companyID, &tokenExpiry, &revokedAt, &replacedBy, &sessionIDValue); err != nil {
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
			if _, err := tx.Exec(ctx, `UPDATE tenant_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE company_id=$1 AND user_id=$2 AND revoked_at IS NULL`, companyID, userID); err != nil {
				return User{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return User{}, err
			}
			return User{}, ErrRefreshReused
		}
		return User{}, ErrRefreshInvalid
	}
	if sessionIDValue == nil {
		return User{}, ErrRefreshInvalid
	}
	sessionID := *sessionIDValue
	var sessionActive bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tenant_auth_sessions WHERE id=$1 AND company_id=$2 AND user_id=$3 AND revoked_at IS NULL AND idle_expires_at>now())`, sessionID, companyID, userID).Scan(&sessionActive); err != nil {
		return User{}, err
	}
	if !sessionActive {
		return User{}, ErrRefreshInvalid
	}

	var newTokenID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO refresh_tokens (company_id, user_id, session_id, token_hash, expires_at, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, companyID, userID, sessionID, newHash, expiresAt, userAgent).Scan(&newTokenID); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $1 WHERE id = $2`, newTokenID, tokenID); err != nil {
		return User{}, err
	}
	var sessionDeadline time.Time
	if err := tx.QueryRow(ctx, `UPDATE tenant_auth_sessions SET last_activity_at=GREATEST(last_activity_at,clock_timestamp()), idle_expires_at=GREATEST(idle_expires_at,$2) WHERE id=$1 AND revoked_at IS NULL AND idle_expires_at>clock_timestamp() RETURNING idle_expires_at`, sessionID, expiresAt).Scan(&sessionDeadline); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return User{}, ErrRefreshInvalid }
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET expires_at=$2 WHERE id=$1`, newTokenID, sessionDeadline); err != nil {
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
	user.SessionID = sessionID
	return user, nil
}

func (r *PostgresRepository) RevokeRefreshToken(ctx context.Context, hash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE tenant_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE id IN (SELECT session_id FROM refresh_tokens WHERE token_hash=$1)`, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) TouchTenantSession(ctx context.Context, sessionID string, userID, companyID int64, refreshHash string, expiresAt time.Time) (bool, bool, error) {
	var sessionDeadline time.Time
	if err := r.pool.QueryRow(ctx, `UPDATE tenant_auth_sessions SET last_activity_at=GREATEST(last_activity_at,clock_timestamp()), idle_expires_at=GREATEST(idle_expires_at,$4) WHERE id=$1 AND user_id=$2 AND company_id=$3 AND revoked_at IS NULL AND idle_expires_at>clock_timestamp() RETURNING idle_expires_at`, sessionID, userID, companyID, expiresAt).Scan(&sessionDeadline); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	if _, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET expires_at=$2 WHERE session_id=$1 AND revoked_at IS NULL`, sessionID, sessionDeadline); err != nil {
		return false, false, err
	}
	var cookieMatches bool
	if refreshHash != "" {
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM refresh_tokens WHERE session_id=$1 AND token_hash=$2 AND revoked_at IS NULL)`, sessionID, refreshHash).Scan(&cookieMatches); err != nil {
			return false, false, err
		}
	}
	return true, cookieMatches, nil
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

func (r *PostgresRepository) CreatePlatformRefreshToken(ctx context.Context, userID int64, sessionID, hash string, expiresAt time.Time, userAgent string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO platform_auth_sessions(id,platform_user_id,last_activity_at,idle_expires_at) VALUES($1,$2,now(),$3)`, sessionID, userID, expiresAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO platform_refresh_tokens(platform_user_id,session_id,token_hash,expires_at,user_agent) VALUES($1,$2,$3,$4,$5)`, userID, sessionID, hash, expiresAt, userAgent); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) RotatePlatformRefreshToken(ctx context.Context, oldHash, newHash string, expiresAt time.Time, userAgent string) (PlatformUser, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return PlatformUser{}, err
	}
	defer tx.Rollback(ctx)

	var tokenID, userID int64
	var sessionIDValue *string
	var tokenExpiry time.Time
	var revokedAt *time.Time
	var replacedBy *int64
	if err := tx.QueryRow(ctx, `
		SELECT id, platform_user_id, expires_at, revoked_at, replaced_by, session_id
		FROM platform_refresh_tokens WHERE token_hash = $1 FOR UPDATE
	`, oldHash).Scan(&tokenID, &userID, &tokenExpiry, &revokedAt, &replacedBy, &sessionIDValue); err != nil {
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
			if _, err := tx.Exec(ctx, `UPDATE platform_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE platform_user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
				return PlatformUser{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return PlatformUser{}, err
			}
			return PlatformUser{}, ErrPlatformRefreshReused
		}
		return PlatformUser{}, ErrPlatformRefreshInvalid
	}
	if sessionIDValue == nil {
		return PlatformUser{}, ErrPlatformRefreshInvalid
	}
	sessionID := *sessionIDValue
	var sessionActive bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_auth_sessions WHERE id=$1 AND platform_user_id=$2 AND revoked_at IS NULL AND idle_expires_at>now())`, sessionID, userID).Scan(&sessionActive); err != nil {
		return PlatformUser{}, err
	}
	if !sessionActive {
		return PlatformUser{}, ErrPlatformRefreshInvalid
	}

	var newTokenID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO platform_refresh_tokens (platform_user_id, session_id, token_hash, expires_at, user_agent)
		VALUES ($1, $2, $3, $4, $5) RETURNING id
	`, userID, sessionID, newHash, expiresAt, userAgent).Scan(&newTokenID); err != nil {
		return PlatformUser{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE platform_refresh_tokens SET revoked_at = now(), replaced_by = $1 WHERE id = $2`, newTokenID, tokenID); err != nil {
		return PlatformUser{}, err
	}
	var sessionDeadline time.Time
	if err := tx.QueryRow(ctx, `UPDATE platform_auth_sessions SET last_activity_at=GREATEST(last_activity_at,clock_timestamp()), idle_expires_at=GREATEST(idle_expires_at,$2) WHERE id=$1 AND revoked_at IS NULL AND idle_expires_at>clock_timestamp() RETURNING idle_expires_at`, sessionID, expiresAt).Scan(&sessionDeadline); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { return PlatformUser{}, ErrPlatformRefreshInvalid }
		return PlatformUser{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE platform_refresh_tokens SET expires_at=$2 WHERE id=$1`, newTokenID, sessionDeadline); err != nil {
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
	user.SessionID = sessionID
	return user, nil
}

func (r *PostgresRepository) RevokePlatformRefreshToken(ctx context.Context, hash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE platform_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE id IN (SELECT session_id FROM platform_refresh_tokens WHERE token_hash=$1)`, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE platform_refresh_tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) TouchPlatformSession(ctx context.Context, sessionID string, userID int64, refreshHash string, expiresAt time.Time) (bool, bool, error) {
	var sessionDeadline time.Time
	if err := r.pool.QueryRow(ctx, `UPDATE platform_auth_sessions SET last_activity_at=GREATEST(last_activity_at,clock_timestamp()), idle_expires_at=GREATEST(idle_expires_at,$3) WHERE id=$1 AND platform_user_id=$2 AND revoked_at IS NULL AND idle_expires_at>clock_timestamp() RETURNING idle_expires_at`, sessionID, userID, expiresAt).Scan(&sessionDeadline); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	if _, err := r.pool.Exec(ctx, `UPDATE platform_refresh_tokens SET expires_at=$2 WHERE session_id=$1 AND revoked_at IS NULL`, sessionID, sessionDeadline); err != nil {
		return false, false, err
	}
	var cookieMatches bool
	if refreshHash != "" {
		if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_refresh_tokens WHERE session_id=$1 AND token_hash=$2 AND revoked_at IS NULL)`, sessionID, refreshHash).Scan(&cookieMatches); err != nil {
			return false, false, err
		}
	}
	return true, cookieMatches, nil
}
