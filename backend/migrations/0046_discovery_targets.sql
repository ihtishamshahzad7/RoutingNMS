-- Feature 1.2 (Device Auto-Discovery), Phase 1 of the RoutingNMS build
-- blueprint. The manual "scan a CIDR, review results, import selected"
-- flow (backend/internal/discovery, Kuma-parity feature "subnet
-- discovery") already existed before this migration and is unchanged --
-- its scan jobs are still deliberately in-memory only (see scanjob.go's
-- own doc comment: an operator-triggered, short-lived UI interaction).
--
-- What was missing, and what this migration adds storage for, is the
-- "auto" half of "auto-discovery": a saved list of subnets to rescan on a
-- schedule in the background, and a durable holding area for hosts that
-- scan turns up so a newly-appeared device survives across scans (and a
-- process restart) until an operator reviews and imports or dismisses it.

CREATE TABLE IF NOT EXISTS discovery_targets (
    id                 BIGSERIAL PRIMARY KEY,
    organization_id    TEXT NOT NULL DEFAULT '',
    cidr               TEXT NOT NULL,
    snmp_version       TEXT NOT NULL DEFAULT '2c',
    snmp_community     TEXT NOT NULL DEFAULT 'public',
    snmp_username      TEXT NOT NULL DEFAULT '',
    snmp_auth_proto    TEXT NOT NULL DEFAULT '',
    snmp_auth_pass     TEXT NOT NULL DEFAULT '',
    snmp_priv_proto    TEXT NOT NULL DEFAULT '',
    snmp_priv_pass     TEXT NOT NULL DEFAULT '',
    snmp_port          INTEGER NOT NULL DEFAULT 161 CHECK (snmp_port > 0 AND snmp_port <= 65535),
    timeout_ms         INTEGER NOT NULL DEFAULT 1500 CHECK (timeout_ms > 0),
    interval_seconds   INTEGER NOT NULL DEFAULT 3600 CHECK (interval_seconds >= 300),
    enabled            BOOLEAN NOT NULL DEFAULT TRUE,
    last_scanned_at    TIMESTAMPTZ,
    last_scan_error    TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_discovery_targets_org ON discovery_targets(organization_id);
CREATE INDEX IF NOT EXISTS idx_discovery_targets_enabled ON discovery_targets(enabled) WHERE enabled;

-- One row per (target, address) ever seen responsive by a scheduled scan
-- of that target. status starts "new" and moves to "imported" (a real
-- devices row was created from it) or "ignored" (operator dismissed it);
-- either way the row is kept for history rather than deleted, mirroring
-- how this codebase generally prefers a status column over a delete
-- elsewhere (e.g. devices.enabled for pause/resume).
CREATE TABLE IF NOT EXISTS discovery_candidates (
    id              BIGSERIAL PRIMARY KEY,
    target_id       BIGINT NOT NULL REFERENCES discovery_targets(id) ON DELETE CASCADE,
    address         TEXT NOT NULL,
    system_name     TEXT NOT NULL DEFAULT '',
    sys_descr       TEXT NOT NULL DEFAULT '',
    sys_object_id   TEXT NOT NULL DEFAULT '',
    device_type     TEXT NOT NULL DEFAULT 'other',
    vendor          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'imported', 'ignored')),
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    imported_device_id TEXT,
    UNIQUE (target_id, address)
);

CREATE INDEX IF NOT EXISTS idx_discovery_candidates_target ON discovery_candidates(target_id);
CREATE INDEX IF NOT EXISTS idx_discovery_candidates_status ON discovery_candidates(status);

COMMENT ON TABLE discovery_targets IS 'Saved subnets rescanned on a schedule by the background auto-discovery poller (Feature 1.2). The pre-existing manual scan-review-import flow (backend/internal/discovery) is unaffected and still works ad hoc without any row here.';
COMMENT ON TABLE discovery_candidates IS 'Hosts a scheduled discovery_targets scan found responsive, held here until an operator imports them as a real device or dismisses them. Not used by the pre-existing manual one-off scan flow, whose results stay in-memory only.';
