-- Phase 0.1 (RoutingNMS build blueprint): RBAC foundation, layered onto the
-- existing session-cookie auth (internal/auth) and the existing `tenants`
-- table (migration 0019) rather than replacing either -- this codebase
-- already scopes most tables by a plain-text tenant_id/organization_id
-- ("" = unattributed, the convention established since feature 18), so
-- roles/permissions follow that same TEXT-tenant-id domain instead of
-- introducing a parallel UUID scheme. Idempotent, like every migration
-- here: safe to re-run on every deployment update.

-- Which tenant a user account defaults to when no more specific context is
-- available (e.g. an MSP operator with roles in several tenants still needs
-- a "home" tenant for UI defaults). '' matches the existing unattributed
-- convention -- an empty default_tenant_id is not an error state.
ALTER TABLE users ADD COLUMN IF NOT EXISTS default_tenant_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS roles (
    id             BIGSERIAL PRIMARY KEY,
    -- '' = a system-wide role (e.g. super_admin), visible/assignable across
    -- every tenant, mirroring how devices.organization_id='' already means
    -- "unattributed" rather than using NULL for that case.
    tenant_id      TEXT NOT NULL DEFAULT '',
    name           TEXT NOT NULL,
    is_system_role BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE IF NOT EXISTS permissions (
    id  BIGSERIAL PRIMARY KEY,
    key TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id   BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    -- A user can hold the same role across different tenants (an MSP
    -- operator managing several customer tenants) -- tenant_id is part of
    -- the key, not just a label on the role.
    tenant_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, role_id, tenant_id)
);
CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles(user_id);

-- Starter permission set covering the resource areas this codebase already
-- has (devices, alerts, OLTs, topology, workspace-topology, config
-- push/backup, tenant/user administration). Additive list -- a future
-- feature can INSERT ... ON CONFLICT DO NOTHING more keys as new resource
-- areas gain enforcement, without a migration bump each time.
INSERT INTO permissions (key) VALUES
    ('device.read'), ('device.write'),
    ('alert.read'), ('alert.write'), ('alert.ack'),
    ('olt.read'), ('olt.write'),
    ('topology.read'), ('topology.write'),
    ('config.read'), ('config.push'),
    ('tenant.admin'), ('user.manage'), ('role.manage'),
    ('report.read')
ON CONFLICT (key) DO NOTHING;

-- super_admin: a system-wide role (tenant_id='') carrying every permission
-- above. This is the role the default admin account is granted below, so
-- an existing single-admin deployment sees no behavior change -- RBAC
-- enforcement is additive capability, not yet wired onto any existing
-- route (see internal/rbac's own doc comment for what is and isn't
-- enforced yet).
INSERT INTO roles (tenant_id, name, is_system_role)
VALUES ('', 'super_admin', true)
ON CONFLICT (tenant_id, name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.tenant_id = '' AND r.name = 'super_admin'
ON CONFLICT DO NOTHING;

-- Grant the bootstrapped default admin account (internal/auth.Store.
-- Bootstrap creates it when users is empty) super_admin, so RBAC-gated
-- routes remain usable immediately on both a fresh install and an existing
-- deployment being upgraded in place. A no-op if no user named "admin"
-- exists (e.g. the operator already renamed/removed it).
INSERT INTO user_roles (user_id, role_id, tenant_id)
SELECT u.id, r.id, ''
FROM users u, roles r
WHERE u.username = 'admin' AND r.tenant_id = '' AND r.name = 'super_admin'
ON CONFLICT DO NOTHING;
