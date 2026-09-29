ALTER TABLE companies
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    ADD COLUMN suspended_at TIMESTAMPTZ,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

INSERT INTO permissions (key, description)
VALUES ('companies.manage', 'List, update, and suspend companies')
ON CONFLICT (key) DO NOTHING;

INSERT INTO platform_role_permissions (platform_role_id, permission_id)
SELECT role.id, permission.id
FROM platform_roles role
JOIN permissions permission ON permission.key = 'companies.manage'
WHERE role.slug = 'product_owner'
ON CONFLICT DO NOTHING;
