INSERT INTO permissions (key, description, audience)
VALUES ('operations.logs.edit', 'Edit work log entries in authorized warehouses', 'company')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key = 'operations.logs.edit'
WHERE role.slug IN ('super_admin', 'admin', 'manager')
ON CONFLICT DO NOTHING;