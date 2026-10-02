CREATE TABLE items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    sku TEXT CHECK (sku IS NULL OR length(sku) <= 64),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id)
);

CREATE UNIQUE INDEX items_company_sku ON items(company_id, lower(sku)) WHERE sku IS NOT NULL;
CREATE INDEX items_company_active_name ON items(company_id, is_active, name, id);

CREATE TABLE item_steps (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    position INT NOT NULL CHECK (position > 0),
    title TEXT NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 120),
    instructions TEXT NOT NULL DEFAULT '' CHECK (length(instructions) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (company_id, item_id) REFERENCES items(company_id, id) ON DELETE CASCADE,
    UNIQUE (company_id, item_id, position)
);

INSERT INTO permissions (key, description, audience) VALUES
    ('items.view', 'View company items and their production steps', 'company'),
    ('items.manage', 'Create, edit, and archive company items and production steps', 'company')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key = 'items.view'
WHERE role.slug IN ('super_admin', 'admin')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key = 'items.manage'
WHERE role.slug IN ('super_admin', 'admin')
ON CONFLICT DO NOTHING;