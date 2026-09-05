-- Two-factor authentication (TOTP) and per-user API keys.
--
-- Mirrors Uptime Kuma's battle-tested model, adapted to RoutingNMS's
-- per-user `users` table (RoutingNMS is multi-operator, unlike Kuma's
-- single-admin-user design, but login/session identity here is still the
-- user row, not the tenant row, so both features attach to `users`).
--
-- Purely additive: every new column has a safe "off"/empty default, so
-- existing accounts are completely unaffected until they opt in.
-- Idempotent by design (IF NOT EXISTS everywhere).

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS twofa_secret     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS twofa_status     BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS twofa_last_token TEXT NOT NULL DEFAULT '';

-- API keys. One row per issued key, scoped to the user who created it
-- (requests authenticated by a key act as that key's owning user for API
-- purposes, same as Kuma). The presented key is shaped
-- "rns_<row id>_<random secret>"; only a hash of the secret portion is
-- stored (never the raw secret) so a database leak alone cannot be used to
-- forge a key. expires_at is nullable: NULL means "never expires", following
-- this codebase's existing nullable-timestamp convention (see
-- devices.last_provisioned_at / provisioning_template_id).
CREATE TABLE IF NOT EXISTS api_keys (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    secret_hash  TEXT NOT NULL,
    active       BOOLEAN NOT NULL DEFAULT true,
    expires_at   TIMESTAMPTZ,
    created_date TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);

-- Short-lived pending-2FA tokens: issued by Login when a user's password
-- checks out but twofa_status is enabled, so the real session token is not
-- issued until a second request proves possession of the authenticator
-- app via LoginTwoFA. Deliberately separate from `sessions` (a pending
-- token is not a valid session and must never authenticate a request).
CREATE TABLE IF NOT EXISTS twofa_pending (
    token_hash TEXT NOT NULL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_twofa_pending_expires_at ON twofa_pending(expires_at);
