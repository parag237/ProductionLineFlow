DELETE FROM platform_role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE key = 'companies.manage');

DELETE FROM permissions WHERE key = 'companies.manage';

ALTER TABLE companies
    DROP COLUMN updated_at,
    DROP COLUMN suspended_at,
    DROP COLUMN status;
