INSERT INTO permissions (key, description, audience)
VALUES ('operations.logs.date_override', 'Record work logs for dates other than today', 'company')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key = 'operations.logs.date_override'
WHERE role.slug IN ('super_admin', 'admin')
ON CONFLICT DO NOTHING;