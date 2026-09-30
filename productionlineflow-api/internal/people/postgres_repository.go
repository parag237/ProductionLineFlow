package people

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

func writeError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" || pgErr.Code == "23503" || pgErr.Code == "23514" {
			return fmt.Errorf("%w: %s", ErrConflict, pgErr.ConstraintName)
		}
	}
	return err
}

const peopleQuery = `
SELECT u.id, u.name, u.email, u.is_active, u.created_at::text,
       a.id, a.role_id, r.slug, r.name, r.scope, a.warehouse_id, COALESCE(w.name, '')
FROM users u
LEFT JOIN user_role_assignments a ON a.user_id = u.id AND a.company_id = u.company_id
LEFT JOIN roles r ON r.id = a.role_id AND r.company_id = a.company_id
LEFT JOIN warehouses w ON w.id = a.warehouse_id AND w.company_id = a.company_id
WHERE u.company_id = $1
  AND ($2::BIGINT IS NULL OR u.id = $2)
  AND ($3::BIGINT[] IS NULL OR (r.slug = 'worker' AND a.warehouse_id = ANY($3)))
  AND ($4 OR u.is_active)
ORDER BY u.name, u.id, r.name, w.name`

func (r *PostgresRepository) ListPeople(ctx context.Context, companyID int64, personID *int64, warehouseIDs []int64, workerOnly, includeInactive bool) ([]Person, error) {
	if workerOnly && warehouseIDs == nil {
		return nil, ErrForbidden
	}
	if !workerOnly {
		warehouseIDs = nil
	}
	rows, err := r.pool.Query(ctx, peopleQuery, companyID, personID, warehouseIDs, includeInactive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := make([]Person, 0)
	indices := map[int64]int{}
	for rows.Next() {
		var person Person
		var assignmentID, roleID, warehouseID *int64
		var roleSlug, roleName, roleScope, warehouseName *string
		if err := rows.Scan(&person.ID, &person.Name, &person.Email, &person.IsActive, &person.CreatedAt,
			&assignmentID, &roleID, &roleSlug, &roleName, &roleScope, &warehouseID, &warehouseName); err != nil {
			return nil, err
		}
		idx, exists := indices[person.ID]
		if !exists {
			person.Assignments = make([]Assignment, 0)
			idx = len(people)
			indices[person.ID] = idx
			people = append(people, person)
		}
		if assignmentID != nil && roleID != nil && roleSlug != nil && roleName != nil && roleScope != nil {
			people[idx].Assignments = append(people[idx].Assignments, Assignment{ID: *assignmentID, RoleID: *roleID, RoleSlug: *roleSlug, RoleName: *roleName, RoleScope: *roleScope, WarehouseID: warehouseID, WarehouseName: valueOf(warehouseName)})
		}
	}
	return people, rows.Err()
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *PostgresRepository) GetPerson(ctx context.Context, companyID, id int64, warehouseIDs []int64, workerOnly bool) (Person, error) {
	var scope []int64
	if workerOnly {
		scope = warehouseIDs
	}
	people, err := r.ListPeople(ctx, companyID, &id, scope, workerOnly, true)
	if err != nil {
		return Person{}, err
	}
	if len(people) == 0 {
		return Person{}, ErrNotFound
	}
	return people[0], nil
}

func (r *PostgresRepository) validateAssignment(ctx context.Context, tx pgx.Tx, companyID int64, input AssignmentInput, policy AssignmentPolicy) (string, string, error) {
	var slug, scope string
	if err := tx.QueryRow(ctx, `SELECT slug, scope FROM roles WHERE id=$1 AND company_id=$2`, input.RoleID, companyID).Scan(&slug, &scope); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}
	if slug == "super_admin" {
		return "", "", ErrForbidden
	}
	if scope == "company" && input.WarehouseID != nil {
		return "", "", ErrInvalidInput
	}
	if scope == "warehouse" && input.WarehouseID == nil {
		return "", "", ErrInvalidInput
	}
	if !policy.SuperAdmin {
		if scope != "warehouse" {
			return "", "", ErrForbidden
		}
		if !policy.CompanyUser && slug != "worker" {
			return "", "", ErrForbidden
		}
		if input.WarehouseID == nil || (!policy.CompanyUser && !containsID(policy.Warehouses, *input.WarehouseID)) {
			return "", "", ErrForbidden
		}
		var belongs bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, *input.WarehouseID, companyID).Scan(&belongs); err != nil {
			return "", "", err
		}
		if !belongs {
			return "", "", ErrInvalidInput
		}
	}
	if scope == "warehouse" {
		var belongs bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM warehouses WHERE id=$1 AND company_id=$2)`, *input.WarehouseID, companyID).Scan(&belongs); err != nil {
			return "", "", err
		}
		if !belongs {
			return "", "", ErrInvalidInput
		}
	}
	return slug, scope, nil
}

func containsID(ids []int64, id int64) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func insertAssignment(ctx context.Context, tx pgx.Tx, companyID, userID int64, input AssignmentInput) (bool, error) {
	var result pgconn.CommandTag
	var err error
	if input.WarehouseID == nil {
		result, err = tx.Exec(ctx, `INSERT INTO user_role_assignments (company_id,user_id,role_id) VALUES ($1,$2,$3) ON CONFLICT (user_id,role_id) WHERE warehouse_id IS NULL DO NOTHING`, companyID, userID, input.RoleID)
	} else {
		result, err = tx.Exec(ctx, `INSERT INTO user_role_assignments (company_id,user_id,role_id,warehouse_id) VALUES ($1,$2,$3,$4) ON CONFLICT (user_id,role_id,warehouse_id) WHERE warehouse_id IS NOT NULL DO NOTHING`, companyID, userID, input.RoleID, *input.WarehouseID)
	}
	return result.RowsAffected() > 0, writeError(err)
}

func (r *PostgresRepository) CreatePerson(ctx context.Context, companyID, creatorID int64, input CreateInput, passwordHash string, policy AssignmentPolicy) (Person, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Person{}, err
	}
	defer tx.Rollback(ctx)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO users(company_id,name,email,password_hash,created_by) VALUES($1,$2,$3,$4,$5) RETURNING id`, companyID, input.Name, input.Email, passwordHash, creatorID).Scan(&id)
	if err != nil {
		return Person{}, writeError(err)
	}
	for _, assignment := range input.Assignments {
		if _, _, err := r.validateAssignment(ctx, tx, companyID, assignment, policy); err != nil {
			return Person{}, err
		}
		if _, err := insertAssignment(ctx, tx, companyID, id, assignment); err != nil {
			return Person{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Person{}, err
	}
	return r.GetPerson(ctx, companyID, id, nil, false)
}

func (r *PostgresRepository) targetIsPrivileged(ctx context.Context, tx pgx.Tx, companyID, userID int64) (bool, error) {
	var privileged bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.company_id=$1 AND a.user_id=$2 AND r.slug IN ('admin','super_admin'))`, companyID, userID).Scan(&privileged)
	return privileged, err
}

func (r *PostgresRepository) UpdatePerson(ctx context.Context, companyID, id int64, input UpdateInput, canManageAdmins bool) (Person, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Person{}, err
	}
	defer tx.Rollback(ctx)
	var lockedCompanyID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM companies WHERE id=$1 FOR UPDATE`, companyID).Scan(&lockedCompanyID); errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrNotFound
	} else if err != nil {
		return Person{}, err
	}
	privileged, err := r.targetIsPrivileged(ctx, tx, companyID, id)
	if err != nil {
		return Person{}, err
	}
	if privileged && !canManageAdmins {
		return Person{}, ErrForbidden
	}
	var currentActive bool
	err = tx.QueryRow(ctx, `SELECT is_active FROM users WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&currentActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrNotFound
	}
	if err != nil {
		return Person{}, err
	}
	if input.IsActive != nil && currentActive != *input.IsActive && !*input.IsActive {
		var assignedSA, activeSAs int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE a.user_id=$2), COUNT(*) FROM user_role_assignments a JOIN roles r ON r.id=a.role_id JOIN users u ON u.id=a.user_id WHERE a.company_id=$1 AND r.slug='super_admin' AND u.is_active`, companyID, id).Scan(&assignedSA, &activeSAs); err != nil {
			return Person{}, err
		}
		if assignedSA > 0 && activeSAs <= 1 {
			return Person{}, ErrLastSuperAdmin
		}
	}
	var updatedActive bool
	err = tx.QueryRow(ctx, `UPDATE users SET name=COALESCE($3,name), email=COALESCE($4,email), is_active=COALESCE($5,is_active), perm_version=CASE WHEN $5::BOOLEAN IS NOT NULL AND $5::BOOLEAN<>is_active THEN perm_version+1 ELSE perm_version END WHERE id=$1 AND company_id=$2 RETURNING is_active`, id, companyID, input.Name, input.Email, input.IsActive).Scan(&updatedActive)
	if err != nil {
		return Person{}, writeError(err)
	}
	if currentActive && !updatedActive {
		if _, err := tx.Exec(ctx, `UPDATE tenant_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE company_id=$1 AND user_id=$2 AND revoked_at IS NULL`, companyID, id); err != nil {
			return Person{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND company_id=$2 AND revoked_at IS NULL`, id, companyID); err != nil {
			return Person{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Person{}, err
	}
	return r.GetPerson(ctx, companyID, id, nil, false)
}

func (r *PostgresRepository) SetPassword(ctx context.Context, companyID, id int64, passwordHash string, canManageAdmins bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	privileged, err := r.targetIsPrivileged(ctx, tx, companyID, id)
	if err != nil {
		return err
	}
	if privileged && !canManageAdmins {
		return ErrForbidden
	}
	result, err := tx.Exec(ctx, `UPDATE users SET password_hash=$3, perm_version=perm_version+1 WHERE id=$1 AND company_id=$2`, id, companyID, passwordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND company_id=$2 AND revoked_at IS NULL`, id, companyID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE tenant_auth_sessions SET revoked_at=COALESCE(revoked_at,now()) WHERE user_id=$1 AND company_id=$2 AND revoked_at IS NULL`, id, companyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) PasswordHash(ctx context.Context, companyID, id int64) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE company_id=$1 AND id=$2 AND is_active`, companyID, id).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

func (r *PostgresRepository) AddAssignment(ctx context.Context, companyID, actorID, userID int64, input AssignmentInput, policy AssignmentPolicy) (Assignment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, err
	}
	defer tx.Rollback(ctx)
	privileged, err := r.targetIsPrivileged(ctx, tx, companyID, userID)
	if err != nil {
		return Assignment{}, err
	}
	if privileged && !policy.SuperAdmin {
		return Assignment{}, ErrForbidden
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT is_active FROM users WHERE id=$1 AND company_id=$2 FOR UPDATE`, userID, companyID).Scan(&active); errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, ErrNotFound
	} else if err != nil {
		return Assignment{}, err
	}
	if !active {
		return Assignment{}, ErrConflict
	}
	_, _, err = r.validateAssignment(ctx, tx, companyID, input, policy)
	if err != nil {
		return Assignment{}, err
	}
	inserted, err := insertAssignment(ctx, tx, companyID, userID, input)
	if err != nil {
		return Assignment{}, err
	}
	if inserted {
		if _, err := tx.Exec(ctx, `UPDATE users SET perm_version=perm_version+1 WHERE id=$1 AND company_id=$2`, userID, companyID); err != nil {
			return Assignment{}, err
		}
	}
	var assignment Assignment
	err = tx.QueryRow(ctx, `SELECT a.id,a.role_id,r.slug,r.name,r.scope,a.warehouse_id,COALESCE(w.name,'') FROM user_role_assignments a JOIN roles r ON r.id=a.role_id LEFT JOIN warehouses w ON w.id=a.warehouse_id WHERE a.company_id=$1 AND a.user_id=$2 AND a.role_id=$3 AND a.warehouse_id IS NOT DISTINCT FROM $4`, companyID, userID, input.RoleID, input.WarehouseID).Scan(&assignment.ID, &assignment.RoleID, &assignment.RoleSlug, &assignment.RoleName, &assignment.RoleScope, &assignment.WarehouseID, &assignment.WarehouseName)
	if err != nil {
		return Assignment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, err
	}
	return assignment, nil
}

func (r *PostgresRepository) RemoveAssignment(ctx context.Context, companyID, actorID, userID, assignmentID int64, policy AssignmentPolicy) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var slug, scope string
	var warehouseID *int64
	err = tx.QueryRow(ctx, `SELECT r.slug,r.scope,a.warehouse_id FROM user_role_assignments a JOIN roles r ON r.id=a.role_id WHERE a.id=$1 AND a.company_id=$2 AND a.user_id=$3 FOR UPDATE`, assignmentID, companyID, userID).Scan(&slug, &scope, &warehouseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if slug == "super_admin" {
		return ErrLastSuperAdmin
	}
	privileged, err := r.targetIsPrivileged(ctx, tx, companyID, userID)
	if err != nil {
		return err
	}
	if privileged && !policy.SuperAdmin {
		return ErrForbidden
	}
	if !policy.SuperAdmin {
		if scope != "warehouse" || warehouseID == nil || (!policy.CompanyUser && !containsID(policy.Warehouses, *warehouseID)) {
			return ErrForbidden
		}
		if !policy.CompanyUser && slug != "worker" {
			return ErrForbidden
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_role_assignments WHERE id=$1`, assignmentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET perm_version=perm_version+1 WHERE id=$1 AND company_id=$2`, userID, companyID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) TransferSuperAdmin(ctx context.Context, companyID, fromID, toID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedCompanyID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM companies WHERE id=$1 FOR UPDATE`, companyID).Scan(&lockedCompanyID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	var targetActive bool
	if err := tx.QueryRow(ctx, `SELECT is_active FROM users WHERE id=$1 AND company_id=$2 FOR UPDATE`, toID, companyID).Scan(&targetActive); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if !targetActive {
		return ErrConflict
	}
	var activeSAs int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM user_role_assignments a JOIN roles r ON r.id=a.role_id JOIN users u ON u.id=a.user_id WHERE a.company_id=$1 AND r.slug='super_admin' AND u.is_active`, companyID).Scan(&activeSAs); err != nil {
		return err
	}
	if activeSAs != 1 {
		return ErrConflict
	}
	var saRoleID, adminRoleID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE company_id=$1 AND slug='super_admin'`, companyID).Scan(&saRoleID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE company_id=$1 AND slug='admin'`, companyID).Scan(&adminRoleID); err != nil {
		return err
	}
	var fromHasSA bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_role_assignments WHERE company_id=$1 AND user_id=$2 AND role_id=$3)`, companyID, fromID, saRoleID).Scan(&fromHasSA); err != nil {
		return err
	}
	if !fromHasSA {
		return ErrForbidden
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_role_assignments WHERE company_id=$1 AND user_id=$2 AND role_id=$3`, companyID, fromID, saRoleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_role_assignments(company_id,user_id,role_id) VALUES($1,$2,$3) ON CONFLICT (user_id,role_id) WHERE warehouse_id IS NULL DO NOTHING`, companyID, fromID, adminRoleID); err != nil {
		return err
	}
	// Super Admin is the target's sole role after a transfer. Remove prior
	// company and warehouse assignments in the same transaction.
	if _, err := tx.Exec(ctx, `DELETE FROM user_role_assignments WHERE company_id=$1 AND user_id=$2`, companyID, toID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_role_assignments(company_id,user_id,role_id) VALUES($1,$2,$3) ON CONFLICT (user_id,role_id) WHERE warehouse_id IS NULL DO NOTHING`, companyID, toID, saRoleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET perm_version=perm_version+1 WHERE company_id=$1 AND id IN ($2,$3)`, companyID, fromID, toID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) ListRoles(ctx context.Context, companyID int64) ([]Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT r.id,r.slug,r.name,r.scope,r.is_system,p.key FROM roles r LEFT JOIN role_permissions rp ON rp.role_id=r.id LEFT JOIN permissions p ON p.id=rp.permission_id WHERE r.company_id=$1 ORDER BY r.name,p.key`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := make([]Role, 0)
	indices := map[int64]int{}
	for rows.Next() {
		var role Role
		var permission *string
		if err := rows.Scan(&role.ID, &role.Slug, &role.Name, &role.Scope, &role.IsSystem, &permission); err != nil {
			return nil, err
		}
		idx, ok := indices[role.ID]
		if !ok {
			role.Permissions = make([]string, 0)
			idx = len(roles)
			indices[role.ID] = idx
			roles = append(roles, role)
		}
		if permission != nil {
			roles[idx].Permissions = append(roles[idx].Permissions, *permission)
		}
	}
	return roles, rows.Err()
}

func (r *PostgresRepository) ListPermissionCatalog(ctx context.Context) ([]Permission, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, description FROM permissions WHERE key NOT IN ('admins.manage','roles.manage') ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Permission, 0)
	for rows.Next() {
		var item Permission
		if err := rows.Scan(&item.Key, &item.Description); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateRole(ctx context.Context, companyID int64, input RoleInput, slug string) (Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Role{}, err
	}
	defer tx.Rollback(ctx)
	var id int64
	if err := tx.QueryRow(ctx, `INSERT INTO roles(company_id,slug,name,scope,is_system) VALUES($1,$2,$3,$4,false) RETURNING id`, companyID, slug, input.Name, input.Scope).Scan(&id); err != nil {
		return Role{}, writeError(err)
	}
	if err := replacePermissions(ctx, tx, id, input.Permissions); err != nil {
		return Role{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Role{}, err
	}
	roles, err := r.ListRoles(ctx, companyID)
	if err != nil {
		return Role{}, err
	}
	for _, role := range roles {
		if role.ID == id {
			return role, nil
		}
	}
	return Role{}, ErrNotFound
}

func replacePermissions(ctx context.Context, tx pgx.Tx, roleID int64, keys []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, key := range keys {
		tag, err := tx.Exec(ctx, `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,id FROM permissions WHERE key=$2 ON CONFLICT DO NOTHING`, roleID, key)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInvalidInput
		}
	}
	return nil
}

func (r *PostgresRepository) UpdateRole(ctx context.Context, companyID, id int64, input RoleUpdateInput) (Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Role{}, err
	}
	defer tx.Rollback(ctx)
	var system bool
	if err := tx.QueryRow(ctx, `SELECT is_system FROM roles WHERE id=$1 AND company_id=$2 FOR UPDATE`, id, companyID).Scan(&system); errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrNotFound
	} else if err != nil {
		return Role{}, err
	}
	if system {
		return Role{}, ErrForbidden
	}
	if input.Name != nil {
		if _, err := tx.Exec(ctx, `UPDATE roles SET name=$3 WHERE id=$1 AND company_id=$2`, id, companyID, *input.Name); err != nil {
			return Role{}, writeError(err)
		}
	}
	if err := replacePermissions(ctx, tx, id, input.Permissions); err != nil {
		return Role{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE users u SET perm_version=perm_version+1 WHERE EXISTS(SELECT 1 FROM user_role_assignments a WHERE a.user_id=u.id AND a.company_id=$1 AND a.role_id=$2)`, companyID, id); err != nil {
		return Role{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Role{}, err
	}
	roles, err := r.ListRoles(ctx, companyID)
	if err != nil {
		return Role{}, err
	}
	for _, role := range roles {
		if role.ID == id {
			return role, nil
		}
	}
	return Role{}, ErrNotFound
}

func (r *PostgresRepository) DeleteRole(ctx context.Context, companyID, id int64) error {
	var system bool
	if err := r.pool.QueryRow(ctx, `SELECT is_system FROM roles WHERE id=$1 AND company_id=$2`, id, companyID).Scan(&system); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if system {
		return ErrForbidden
	}
	var assigned bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_role_assignments WHERE company_id=$1 AND role_id=$2)`, companyID, id).Scan(&assigned); err != nil {
		return err
	}
	if assigned {
		return ErrConflict
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id=$1 AND company_id=$2`, id, companyID); err != nil {
		return writeError(err)
	}
	return nil
}
