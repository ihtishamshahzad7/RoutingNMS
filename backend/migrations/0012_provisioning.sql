-- RouterOS auto-provisioning: named script templates rendered per-device,
-- fetched by the router itself via /tool fetch using a deterministic,
-- derived (not stored) password. Admin pre-registers the device (with its
-- serial number) and assigns a template before the router's scheduled
-- fetch can succeed -- there is no unknown-device auto-registration.

CREATE TABLE IF NOT EXISTS provisioning_templates (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    script_body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE devices ADD COLUMN IF NOT EXISTS provisioning_template_id BIGINT REFERENCES provisioning_templates(id) ON DELETE SET NULL;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS last_provisioned_at TIMESTAMPTZ;

INSERT INTO provisioning_templates (name, script_body)
SELECT 'Default RouterOS baseline',
$TPL$/system identity set name="{{.Hostname}}"
/ip service disable telnet,ftp,www
/ip service set ssh port=22
/user set 0 name=admin password="{{.Password}}"
/system ntp client set enabled=yes primary-ntp=pool.ntp.org
/system logging action set 0 memory-lines=1000
{{if .Address}}/ip address add address={{.Address}}/32 interface=ether1 comment="managed by RoutingNMS"{{end}}
:log info "RoutingNMS provisioning applied"
$TPL$
WHERE NOT EXISTS (SELECT 1 FROM provisioning_templates WHERE name = 'Default RouterOS baseline');
