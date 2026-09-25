-- Link validation for the Workspace Topology Builder (feature 26 backlog
-- item "link validation", flagged as an open follow-up since feature 32).
-- A canvas link's sourcePort/targetPort are free-text strings the user (or
-- the discovery engine, feature 33) typed/picked -- nothing has ever
-- checked they correspond to a real, operationally-up interface on the
-- linked real device. This adds storage for the last validation result so
-- the canvas can render a link's real state without re-querying SNMP on
-- every page load.

ALTER TABLE workspace_topology_links
    ADD COLUMN IF NOT EXISTS validation_status TEXT NOT NULL DEFAULT 'unverified',
    ADD COLUMN IF NOT EXISTS validation_detail TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS validated_at TIMESTAMPTZ;

-- validation_status values (checked in Go, not a DB CHECK constraint, to
-- match this codebase's existing convention for status-like text columns
-- e.g. statuspage.ItemStatus.Status, ping's up/down/pending):
--   'unverified' -- default; not yet checked, or an endpoint isn't linked
--                   to a real SNMP-enabled device on both ends
--   'up'         -- both ports found on their real device and operationally up
--   'down'       -- both ports found but at least one is administratively
--                   or operationally down
--   'not_found'  -- a linked device responded to SNMP but no interface
--                   matched the stored port name/index
--   'error'      -- SNMP walk failed (device unreachable, SNMP disabled,
--                   timeout) -- distinct from 'not_found' so the UI can
--                   tell "couldn't check" apart from "checked, doesn't exist"
