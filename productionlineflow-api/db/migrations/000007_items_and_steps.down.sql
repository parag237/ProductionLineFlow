DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key IN ('items.view', 'items.manage'));

DELETE FROM permissions WHERE key IN ('items.view', 'items.manage');

DROP TABLE IF EXISTS item_steps;
DROP TABLE IF EXISTS items;