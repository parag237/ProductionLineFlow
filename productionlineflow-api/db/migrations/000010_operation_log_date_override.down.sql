DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key = 'operations.logs.date_override');

DELETE FROM permissions WHERE key = 'operations.logs.date_override';