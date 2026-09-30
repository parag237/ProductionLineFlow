DROP INDEX IF EXISTS platform_refresh_tokens_session_active;
DROP INDEX IF EXISTS refresh_tokens_session_active;
ALTER TABLE platform_refresh_tokens DROP CONSTRAINT IF EXISTS platform_refresh_tokens_session_required;
ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_session_required;
ALTER TABLE platform_refresh_tokens DROP COLUMN IF EXISTS session_id;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS platform_auth_sessions;
DROP TABLE IF EXISTS tenant_auth_sessions;

