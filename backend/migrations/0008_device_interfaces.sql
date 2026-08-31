CREATE TABLE IF NOT EXISTS interfaces (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    if_index BIGINT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    admin_up BOOLEAN NOT NULL DEFAULT FALSE,
    oper_up BOOLEAN NOT NULL DEFAULT FALSE,
    in_octets BIGINT NOT NULL DEFAULT 0,
    out_octets BIGINT NOT NULL DEFAULT 0,
    in_errors BIGINT NOT NULL DEFAULT 0,
    out_errors BIGINT NOT NULL DEFAULT 0,
    last_discovered_at TIMESTAMPTZ,
    UNIQUE (device_id, if_index)
);

CREATE INDEX IF NOT EXISTS idx_interfaces_device ON interfaces(device_id);
CREATE INDEX IF NOT EXISTS idx_interfaces_oper_up ON interfaces(device_id, oper_up);
