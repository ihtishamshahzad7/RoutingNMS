-- Group-wise, port-level topology link mapping: a distinct feature from the
-- existing auto-generated LLDP topology (internal/topology, topology_links
-- table from migration 0014) -- that feature discovers links automatically
-- and has no concept of an operator-defined group or a named interface on
-- each end. This is the manual complement: an operator explicitly states
-- "device A interface ethX is connected to device B interface ethY",
-- organized into named groups, and RoutingNMS polls each end's SNMP
-- ifOperStatus and alerts by name ("group X device Y port Z down") through
-- the same alertsfeed pipeline as every other monitor type.

CREATE TABLE IF NOT EXISTS topo_link_groups (
	id BIGSERIAL PRIMARY KEY,
	organization_id TEXT NOT NULL,
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_topo_link_groups_org ON topo_link_groups(organization_id);

-- One row covers one bidirectional physical/logical connection: device A's
-- interface_a is connected to device B's interface_b. Interface names are
-- free text (matched case-insensitively against ifDescr/ifName at poll
-- time) rather than a foreign key to a live interface-enumeration table,
-- since no such table exists yet in this codebase -- a documented v1
-- simplification, not a missing requirement.
CREATE TABLE IF NOT EXISTS topo_links (
	id BIGSERIAL PRIMARY KEY,
	group_id BIGINT NOT NULL REFERENCES topo_link_groups(id) ON DELETE CASCADE,
	device_a_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	interface_a TEXT NOT NULL,
	device_b_id BIGINT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
	interface_b TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_topo_links_group ON topo_links(group_id);
CREATE INDEX IF NOT EXISTS idx_topo_links_device_a ON topo_links(device_a_id);
CREATE INDEX IF NOT EXISTS idx_topo_links_device_b ON topo_links(device_b_id);
