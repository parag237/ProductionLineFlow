DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key = 'operations.logs.edit');

DELETE FROM permissions WHERE key = 'operations.logs.edit';