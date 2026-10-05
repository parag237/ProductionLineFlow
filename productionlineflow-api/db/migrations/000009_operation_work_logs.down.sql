DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key IN ('operations.logs.view', 'operations.logs.create'));

DELETE FROM permissions WHERE key IN ('operations.logs.view', 'operations.logs.create');

DROP TABLE IF EXISTS operation_work_logs;
DROP FUNCTION IF EXISTS validate_operation_work_log_step();
DROP INDEX IF EXISTS warehouses_company_id_id;
DROP INDEX IF EXISTS users_company_id_id;