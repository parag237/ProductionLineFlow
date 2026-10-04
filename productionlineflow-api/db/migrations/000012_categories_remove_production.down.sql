CREATE TABLE production_runs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    item_id BIGINT NOT NULL,
    item_name TEXT NOT NULL,
    tracking_mode TEXT NOT NULL CHECK (tracking_mode IN ('batch', 'unit')),
    quantity INT NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed', 'cancelled')),
    created_by BIGINT NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (company_id, item_id) REFERENCES items(company_id, id),
    UNIQUE (company_id, id)
);
CREATE INDEX production_runs_company_created ON production_runs(company_id, created_at DESC, id DESC);

CREATE TABLE production_run_steps (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL,
    run_id BIGINT NOT NULL,
    position INT NOT NULL CHECK (position > 0),
    title TEXT NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    UNIQUE (company_id, run_id, id),
    UNIQUE (company_id, run_id, position),
    FOREIGN KEY (company_id, run_id) REFERENCES production_runs(company_id, id) ON DELETE CASCADE
);

CREATE TABLE production_units (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL,
    run_id BIGINT NOT NULL,
    serial_number TEXT NOT NULL CHECK (length(btrim(serial_number)) BETWEEN 1 AND 120),
    status TEXT NOT NULL DEFAULT 'in_progress' CHECK (status IN ('in_progress', 'completed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (company_id, run_id) REFERENCES production_runs(company_id, id) ON DELETE CASCADE,
    UNIQUE (company_id, id),
    UNIQUE (company_id, run_id, id),
    UNIQUE (company_id, run_id, serial_number)
);
CREATE UNIQUE INDEX production_units_company_serial ON production_units(company_id, lower(serial_number));

CREATE TABLE production_step_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL,
    run_id BIGINT NOT NULL,
    run_step_id BIGINT NOT NULL,
    unit_id BIGINT,
    event TEXT NOT NULL CHECK (event IN ('started', 'completed')),
    actor_id BIGINT NOT NULL REFERENCES users(id),
    note TEXT NOT NULL DEFAULT '' CHECK (length(note) <= 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (company_id, run_id, run_step_id) REFERENCES production_run_steps(company_id, run_id, id) ON DELETE CASCADE,
    FOREIGN KEY (company_id, run_id, unit_id) REFERENCES production_units(company_id, run_id, id)
);
CREATE UNIQUE INDEX production_batch_step_completed ON production_step_events(run_id, run_step_id)
    WHERE unit_id IS NULL AND event = 'completed';
CREATE UNIQUE INDEX production_unit_step_completed ON production_step_events(run_id, unit_id, run_step_id)
    WHERE unit_id IS NOT NULL AND event = 'completed';
CREATE INDEX production_step_events_run ON production_step_events(company_id, run_id, created_at, id);

INSERT INTO permissions (key, description, audience) VALUES
    ('production.view', 'View company production runs and step history', 'company'),
    ('production.execute', 'Start production runs and record step progress', 'company')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key IN ('production.view', 'production.execute')
WHERE role.slug IN ('super_admin', 'admin')
ON CONFLICT DO NOTHING;

ALTER TABLE items DROP CONSTRAINT IF EXISTS items_company_category_fk;
DROP INDEX IF EXISTS items_company_category;
ALTER TABLE items DROP COLUMN IF EXISTS category_id;

DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key IN ('items.categories.view', 'items.categories.manage'));
DELETE FROM permissions WHERE key IN ('items.categories.view', 'items.categories.manage');
DROP TABLE IF EXISTS item_categories;