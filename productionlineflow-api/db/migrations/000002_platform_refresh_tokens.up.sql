CREATE TABLE platform_refresh_tokens (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    platform_user_id BIGINT NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by BIGINT REFERENCES platform_refresh_tokens(id),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX platform_refresh_tokens_user_active
    ON platform_refresh_tokens (platform_user_id)
    WHERE revoked_at IS NULL;
