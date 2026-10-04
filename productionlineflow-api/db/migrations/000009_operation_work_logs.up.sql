CREATE UNIQUE INDEX users_company_id_id ON users (company_id, id);
CREATE UNIQUE INDEX warehouses_company_id_id ON warehouses (company_id, id);

CREATE TABLE operation_work_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    warehouse_id BIGINT NOT NULL,
    work_date DATE NOT NULL,
    item_id BIGINT NOT NULL,
    item_name TEXT NOT NULL,
    step_id BIGINT NOT NULL CHECK (step_id > 0),
    step_title TEXT NOT NULL,
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    performed_by BIGINT NOT NULL,
    recorded_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (company_id, warehouse_id) REFERENCES warehouses(company_id, id),
    FOREIGN KEY (company_id, item_id) REFERENCES items(company_id, id),
    FOREIGN KEY (company_id, performed_by) REFERENCES users(company_id, id),
    FOREIGN KEY (company_id, recorded_by) REFERENCES users(company_id, id)
);

CREATE INDEX operation_work_logs_company_date ON operation_work_logs(company_id, work_date DESC, id DESC);
CREATE INDEX operation_work_logs_warehouse_date ON operation_work_logs(company_id, warehouse_id, work_date DESC, id DESC);

CREATE OR REPLACE FUNCTION validate_operation_work_log_step()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM item_steps
        WHERE company_id = NEW.company_id
          AND item_id = NEW.item_id
          AND id = NEW.step_id
    ) THEN
        RAISE EXCEPTION 'work log step does not belong to item';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER operation_work_log_step_integrity
BEFORE INSERT OR UPDATE OF company_id, item_id, step_id ON operation_work_logs
FOR EACH ROW EXECUTE FUNCTION validate_operation_work_log_step();

INSERT INTO permissions (key, description, audience) VALUES
    ('operations.logs.view', 'View warehouse work logs', 'company'),
    ('operations.logs.create', 'Record warehouse work logs', 'company')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key IN ('operations.logs.view', 'operations.logs.create')
WHERE role.slug IN ('super_admin', 'admin', 'manager')
ON CONFLICT DO NOTHING;