package hostmetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// EnabledDevice is the subset of devices the host-metrics poller iterates --
// same universe as internal/ifpoll's EnabledDevice (enabled, SNMP-enabled
// devices), plus vendor (to pick the right OID set) and the per-device
// alert thresholds item 3.5 added in migration 0053.
type EnabledDevice struct {
	ID                      string
	OrganizationID          string
	Vendor                  string
	Address                 string
	SNMPPort                uint16
	Credentials             snmp.Credentials
	Timeout                 time.Duration
	CPUAlertThresholdPct    int
	MemoryAlertThresholdPct int
}

type Repository struct {
	DB *pgxpool.Pool
}

func (r Repository) ListEnabled(ctx context.Context) ([]EnabledDevice, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("hostmetrics repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,COALESCE(organization_id,''),COALESCE(vendor,''),address,COALESCE(snmp_port,161),snmp_version,COALESCE(snmp_community,''),COALESCE(snmp_username,''),COALESCE(snmp_auth_protocol,''),COALESCE(snmp_auth_password,''),COALESCE(snmp_priv_protocol,''),COALESCE(snmp_priv_password,''),COALESCE(snmp_timeout_ms,3000),cpu_alert_threshold_pct,memory_alert_threshold_pct
		FROM devices WHERE enabled=true AND snmp_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnabledDevice{}
	for rows.Next() {
		var d EnabledDevice
		var version string
		var timeoutMS int
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Vendor, &d.Address, &d.SNMPPort, &version, &d.Credentials.Community, &d.Credentials.Username, &d.Credentials.AuthProto, &d.Credentials.AuthPass, &d.Credentials.PrivProto, &d.Credentials.PrivPass, &timeoutMS, &d.CPUAlertThresholdPct, &d.MemoryAlertThresholdPct); err != nil {
			return nil, err
		}
		d.Credentials.Version = snmp.Version(version)
		d.Timeout = time.Duration(timeoutMS) * time.Millisecond
		out = append(out, d)
	}
	return out, rows.Err()
}
