DROP TRIGGER IF EXISTS platform_role_permission_audience_integrity ON platform_role_permissions;
DROP TRIGGER IF EXISTS role_permission_audience_integrity ON role_permissions;
DROP FUNCTION IF EXISTS validate_platform_role_permission_audience();
DROP FUNCTION IF EXISTS validate_role_permission_audience();
DROP INDEX IF EXISTS permissions_audience_key;
ALTER TABLE permissions DROP COLUMN IF EXISTS audience;
