CREATE TABLE item_categories (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, id)
);

CREATE UNIQUE INDEX item_categories_company_name ON item_categories(company_id, lower(name));
CREATE INDEX item_categories_company_name_order ON item_categories(company_id, name, id);

INSERT INTO item_categories (company_id, name)
SELECT DISTINCT item.company_id, 'Uncategorized'
FROM items item
WHERE NOT EXISTS (
    SELECT 1 FROM item_categories category
    WHERE category.company_id = item.company_id AND lower(category.name) = 'uncategorized'
);

ALTER TABLE items ADD COLUMN category_id BIGINT;

UPDATE items item
SET category_id = category.id
FROM item_categories category
WHERE category.company_id = item.company_id
  AND lower(category.name) = 'uncategorized';

ALTER TABLE items ALTER COLUMN category_id SET NOT NULL;
ALTER TABLE items
    ADD CONSTRAINT items_company_category_fk
    FOREIGN KEY (company_id, category_id) REFERENCES item_categories(company_id, id);
CREATE INDEX items_company_category ON items(company_id, category_id, name, id);

INSERT INTO permissions (key, description, audience) VALUES
    ('items.categories.view', 'View and select item categories', 'company'),
    ('items.categories.manage', 'Create, edit, and delete item categories', 'company')
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT DISTINCT role.id, permission.id
FROM roles role
JOIN role_permissions existing ON existing.role_id = role.id
JOIN permissions item_manage ON item_manage.id = existing.permission_id AND item_manage.key = 'items.manage'
JOIN permissions permission ON permission.key = 'items.categories.view'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key = 'items.categories.manage'
WHERE role.slug IN ('super_admin', 'admin')
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE key IN ('production.view', 'production.execute'));
DELETE FROM permissions WHERE key IN ('production.view', 'production.execute');

DROP TABLE IF EXISTS production_step_events;
DROP TABLE IF EXISTS production_units;
DROP TABLE IF EXISTS production_run_steps;
DROP TABLE IF EXISTS production_runs;