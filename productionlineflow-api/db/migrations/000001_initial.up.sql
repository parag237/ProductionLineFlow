CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE companies (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE platform_users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    email CITEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    perm_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name TEXT NOT NULL,
    email CITEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    perm_version INT NOT NULL DEFAULT 1,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_company_email ON users (company_id, email);

CREATE TABLE permissions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL
);

CREATE TABLE platform_roles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    is_system BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE platform_role_permissions (
    platform_role_id BIGINT NOT NULL REFERENCES platform_roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (platform_role_id, permission_id)
);

CREATE TABLE platform_user_role_assignments (
    platform_user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    platform_role_id BIGINT NOT NULL REFERENCES platform_roles(id),
    PRIMARY KEY (platform_user_id, platform_role_id)
);

CREATE TABLE warehouse_types (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    name TEXT NOT NULL,
    UNIQUE (company_id, name)
);

CREATE TABLE warehouses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    type_id BIGINT NOT NULL REFERENCES warehouse_types(id),
    name TEXT NOT NULL,
    address TEXT,
    state TEXT NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, name)
);

CREATE TABLE roles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('company', 'warehouse')),
    is_system BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (company_id, slug)
);

CREATE TABLE role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_role_assignments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    role_id BIGINT NOT NULL REFERENCES roles(id),
    warehouse_id BIGINT REFERENCES warehouses(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ura_company_scoped
    ON user_role_assignments (user_id, role_id)
    WHERE warehouse_id IS NULL;

CREATE UNIQUE INDEX ura_warehouse_scoped
    ON user_role_assignments (user_id, role_id, warehouse_id)
    WHERE warehouse_id IS NOT NULL;

CREATE INDEX user_role_assignments_user_id ON user_role_assignments (user_id);
CREATE INDEX user_role_assignments_warehouse_id ON user_role_assignments (warehouse_id);

CREATE OR REPLACE FUNCTION validate_user_role_assignment()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    user_company_id BIGINT;
    role_company_id BIGINT;
    role_scope TEXT;
    warehouse_company_id BIGINT;
BEGIN
    SELECT company_id INTO user_company_id FROM users WHERE id = NEW.user_id;
    SELECT company_id, scope INTO role_company_id, role_scope FROM roles WHERE id = NEW.role_id;

    IF user_company_id IS NULL OR role_company_id IS NULL THEN
        RAISE EXCEPTION 'user and role must exist';
    END IF;

    IF NEW.company_id <> user_company_id OR NEW.company_id <> role_company_id THEN
        RAISE EXCEPTION 'assignment company does not match user and role';
    END IF;

    IF role_scope = 'company' AND NEW.warehouse_id IS NOT NULL THEN
        RAISE EXCEPTION 'company-scoped roles cannot have a warehouse';
    END IF;

    IF role_scope = 'warehouse' AND NEW.warehouse_id IS NULL THEN
        RAISE EXCEPTION 'warehouse-scoped roles require a warehouse';
    END IF;

    IF NEW.warehouse_id IS NOT NULL THEN
        SELECT company_id INTO warehouse_company_id FROM warehouses WHERE id = NEW.warehouse_id;
        IF warehouse_company_id IS NULL OR warehouse_company_id <> NEW.company_id THEN
            RAISE EXCEPTION 'assignment warehouse does not match company';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER user_role_assignment_integrity
BEFORE INSERT OR UPDATE ON user_role_assignments
FOR EACH ROW EXECUTE FUNCTION validate_user_role_assignment();

CREATE TABLE flow_sessions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    flow_type TEXT NOT NULL,
    state TEXT NOT NULL,
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    version INT NOT NULL DEFAULT 1,
    completed BOOLEAN NOT NULL DEFAULT false,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX flow_sessions_user_active
    ON flow_sessions (user_id)
    WHERE completed = false;

CREATE TABLE refresh_tokens (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by BIGINT REFERENCES refresh_tokens(id),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_active
    ON refresh_tokens (user_id)
    WHERE revoked_at IS NULL;

INSERT INTO permissions (key, description) VALUES
    ('companies.create', 'Create a company and provision its initial Super Admin'),
    ('admins.manage', 'Create, edit, delete, and assign company administrators'),
    ('roles.manage', 'Manage company roles and role permissions'),
    ('users.create', 'Create company-level users'),
    ('warehouse_types.manage', 'Create, edit, and delete warehouse types'),
    ('warehouses.manage', 'Create, edit, archive, and transition warehouses'),
    ('warehouse.members.manage', 'Assign warehouse-scoped roles in a manageable warehouse'),
    ('warehouse.view', 'View a warehouse and its members'),
    ('workers.tasks.execute', 'Execute operational worker actions'),
    ('stock.view', 'View inventory'),
    ('stock.adjust', 'Adjust inventory')
ON CONFLICT (key) DO NOTHING;

INSERT INTO platform_roles (slug, is_system)
VALUES ('product_owner', true)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO platform_role_permissions (platform_role_id, permission_id)
SELECT r.id, p.id
FROM platform_roles r
JOIN permissions p ON p.key = 'companies.create'
WHERE r.slug = 'product_owner'
ON CONFLICT DO NOTHING;