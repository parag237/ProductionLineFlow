DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key IN ('production.view', 'production.execute'));

DELETE FROM permissions WHERE key IN ('production.view', 'production.execute');

DROP TABLE IF EXISTS production_step_events;
DROP TABLE IF EXISTS production_units;
DROP TABLE IF EXISTS production_run_steps;
DROP TABLE IF EXISTS production_runs;