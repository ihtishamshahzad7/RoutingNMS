-- Phase 0.2 (RoutingNMS build blueprint): secrets encryption at rest.
-- No schema change is required -- notification_channels.config is already
-- JSONB (migration 0017) and stays JSONB; what changes is only the shape of
-- the value the application stores inside it (see backend/internal/secrets
-- and internal/alerts.Repository.SaveChannel/ListChannels). This migration
-- exists purely to document that on the schema itself, matching this
-- project's convention of every gap getting a migration file, and to close
-- the loop on migration 0017's own header comment, which already flagged
-- this exact gap: "Secrets live in `config` JSON and should be encrypted at
-- rest; this schema only stores the container."
--
-- Encryption is opt-in: set ROUTINGNMS_SECRETS_KEY (a base64-encoded
-- 32-byte AES-256 key, e.g. `openssl rand -base64 32`) as an environment
-- variable for the API service to enable it. Unset, nothing changes.
-- Idempotent, like every migration here (COMMENT ON is inherently
-- re-runnable).
COMMENT ON COLUMN notification_channels.config IS
    'Per-provider notification settings (webhook URLs, tokens, credentials). '
    'Encrypted at rest as a single opaque {"__enc":"enc:v1:..."} blob when '
    'the API service has ROUTINGNMS_SECRETS_KEY set (see internal/secrets); '
    'plain JSON otherwise or for rows written before a key was configured.';
