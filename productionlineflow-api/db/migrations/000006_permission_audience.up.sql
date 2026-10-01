ALTER TABLE permissions
    ADD COLUMN audience TEXT NOT NULL DEFAULT 'company'
    CHECK (audience IN ('platform', 'company'));

UPDATE permissions
SET audience = 'platform'
WHERE key IN ('companies.create', 'companies.manage');

-- Remove any cross-audience assignments that may predate this migration.
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE audience = 'platform');

DELETE FROM platform_role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE audience = 'company');

CREATE INDEX permissions_audience_key ON permissions (audience, key);

CREATE OR REPLACE FUNCTION validate_role_permission_audience()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    permission_audience TEXT;
BEGIN
    SELECT audience INTO permission_audience FROM permissions WHERE id = NEW.permission_id;
    IF permission_audience IS DISTINCT FROM 'company' THEN
        RAISE EXCEPTION 'company roles may only use company permissions';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER role_permission_audience_integrity
BEFORE INSERT OR UPDATE OF permission_id ON role_permissions
FOR EACH ROW EXECUTE FUNCTION validate_role_permission_audience();

CREATE OR REPLACE FUNCTION validate_platform_role_permission_audience()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    permission_audience TEXT;
BEGIN
    SELECT audience INTO permission_audience FROM permissions WHERE id = NEW.permission_id;
    IF permission_audience IS DISTINCT FROM 'platform' THEN
        RAISE EXCEPTION 'platform roles may only use platform permissions';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER platform_role_permission_audience_integrity
BEFORE INSERT OR UPDATE OF permission_id ON platform_role_permissions
FOR EACH ROW EXECUTE FUNCTION validate_platform_role_permission_audience();
