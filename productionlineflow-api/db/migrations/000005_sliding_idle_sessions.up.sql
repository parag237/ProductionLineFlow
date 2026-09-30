CREATE TABLE tenant_auth_sessions (
    id TEXT PRIMARY KEY,
    company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_activity_at TIMESTAMPTZ NOT NULL,
    idle_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tenant_auth_sessions_user_active
    ON tenant_auth_sessions (company_id, user_id, idle_expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE platform_auth_sessions (
    id TEXT PRIMARY KEY,
    platform_user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    last_activity_at TIMESTAMPTZ NOT NULL,
    idle_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX platform_auth_sessions_user_active
    ON platform_auth_sessions (platform_user_id, idle_expires_at)
    WHERE revoked_at IS NULL;

ALTER TABLE refresh_tokens ADD COLUMN session_id TEXT REFERENCES tenant_auth_sessions(id) ON DELETE CASCADE;
ALTER TABLE platform_refresh_tokens ADD COLUMN session_id TEXT REFERENCES platform_auth_sessions(id) ON DELETE CASCADE;

-- Legacy access tokens do not carry a session ID. Require one fresh sign-in after deployment.
UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE session_id IS NULL;
UPDATE platform_refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE session_id IS NULL;

ALTER TABLE refresh_tokens ADD CONSTRAINT refresh_tokens_session_required
    CHECK (revoked_at IS NOT NULL OR session_id IS NOT NULL);
ALTER TABLE platform_refresh_tokens ADD CONSTRAINT platform_refresh_tokens_session_required
    CHECK (revoked_at IS NOT NULL OR session_id IS NOT NULL);

CREATE INDEX refresh_tokens_session_active
    ON refresh_tokens (session_id)
    WHERE revoked_at IS NULL;
CREATE INDEX platform_refresh_tokens_session_active
    ON platform_refresh_tokens (session_id)
    WHERE revoked_at IS NULL;

