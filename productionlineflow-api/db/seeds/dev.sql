BEGIN;

DO $$
DECLARE
    v_company_id BIGINT;
    v_role_id BIGINT;
    v_user_id BIGINT;
BEGIN
    INSERT INTO companies (slug, name)
    VALUES ('acme', 'Acme Manufacturing')
    ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
    RETURNING id INTO v_company_id;

    INSERT INTO roles (company_id, slug, name, scope, is_system)
    VALUES (v_company_id, 'super_admin', 'Super Admin', 'company', true)
    ON CONFLICT (company_id, slug) DO UPDATE
        SET name = EXCLUDED.name, scope = EXCLUDED.scope, is_system = EXCLUDED.is_system
    RETURNING id INTO v_role_id;

    INSERT INTO users (company_id, name, email, password_hash, is_active)
    VALUES (
        v_company_id,
        'Demo Admin',
        'admin@acme.test',
        '$argon2id$v=19$m=65536,t=3,p=2$ZyTJ/DXrf1+O9fk3x1tUbA$ly4L0nY5LMamvjtsHd1MtFt+yfDXvLMpigkp4cBLS4s',
        true
    )
    ON CONFLICT (company_id, email) DO UPDATE
        SET name = EXCLUDED.name, password_hash = EXCLUDED.password_hash, is_active = true
    RETURNING id INTO v_user_id;

    INSERT INTO role_permissions (role_id, permission_id)
    SELECT v_role_id, permissions.id
    FROM permissions
    WHERE permissions.key IN (
        'admins.manage',
        'roles.manage',
        'users.create',
        'warehouse_types.manage',
        'warehouses.manage',
        'warehouse.members.manage',
        'warehouse.view',
        'workers.tasks.execute'
    )
    ON CONFLICT DO NOTHING;

    INSERT INTO user_role_assignments (company_id, user_id, role_id)
    VALUES (v_company_id, v_user_id, v_role_id)
    ON CONFLICT DO NOTHING;
END $$;

COMMIT;

BEGIN;

DO $$
DECLARE
    v_platform_user_id BIGINT;
    v_platform_role_id BIGINT;
BEGIN
    INSERT INTO platform_users (name, email, password_hash, is_active)
    VALUES (
        'Demo Product Owner',
        'owner@platform.test',
        '$argon2id$v=19$m=65536,t=3,p=2$ZyTJ/DXrf1+O9fk3x1tUbA$ly4L0nY5LMamvjtsHd1MtFt+yfDXvLMpigkp4cBLS4s',
        true
    )
    ON CONFLICT (email) DO UPDATE
        SET name = EXCLUDED.name, password_hash = EXCLUDED.password_hash, is_active = true
    RETURNING id INTO v_platform_user_id;

    SELECT id INTO v_platform_role_id
    FROM platform_roles
    WHERE slug = 'product_owner';

    INSERT INTO platform_user_role_assignments (platform_user_id, platform_role_id)
    VALUES (v_platform_user_id, v_platform_role_id)
    ON CONFLICT DO NOTHING;
END $$;

COMMIT;
