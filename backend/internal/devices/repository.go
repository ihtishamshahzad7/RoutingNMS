package devices

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

type Record struct {
	ID                        string `json:"id"`
	OrganizationID            string `json:"organizationId"`
	Name                      string `json:"name"`
	Address                   string `json:"address"`
	DeviceType                string `json:"deviceType"`
	Vendor                    string `json:"vendor,omitempty"`
	Model                     string `json:"model,omitempty"`
	SerialNumber              string `json:"serialNumber,omitempty"`
	Enabled                   bool   `json:"enabled"`
	MonitoringIntervalSeconds int    `json:"monitoringIntervalSeconds"`
	SNMPEnabled               bool   `json:"snmpEnabled"`
	SNMPVersion               string `json:"snmpVersion"`
	SNMPPort                  int    `json:"snmpPort"`
	SNMPConfigured            bool   `json:"snmpConfigured"`
}

func (r Repository) Create(ctx context.Context, in DeviceInput) (Record, error) {
	if r.DB == nil {
		return Record{}, fmt.Errorf("device repository is not initialized")
	}
	if in.SNMPPort == 0 {
		in.SNMPPort = 161
	}
	if in.Timeout <= 0 {
		in.Timeout = 3 * time.Second
	}
	var out Record
	err := r.DB.QueryRow(ctx, `INSERT INTO devices (organization_id,name,address,device_type,vendor,enabled,snmp_enabled,snmp_version,snmp_community,snmp_username,snmp_auth_protocol,snmp_auth_password,snmp_priv_protocol,snmp_priv_password,snmp_port,snmp_timeout_ms) VALUES ($1,$2,$3,$4,$5,true,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port`, in.OrganizationID, in.Name, in.Address, in.DeviceType, in.Vendor, in.SNMP.Version != "", in.SNMP.Version, in.SNMP.Community, in.SNMP.Username, in.SNMP.AuthProto, in.SNMP.AuthPass, in.SNMP.PrivProto, in.SNMP.PrivPass, in.SNMPPort, int(in.Timeout/time.Millisecond)).Scan(&out.ID, &out.OrganizationID, &out.Name, &out.Address, &out.DeviceType, &out.Vendor, &out.Model, &out.SerialNumber, &out.Enabled, &out.MonitoringIntervalSeconds, &out.SNMPEnabled, &out.SNMPVersion, &out.SNMPPort)
	out.SNMPConfigured = out.SNMPEnabled
	return out, err
}

func (r Repository) List(ctx context.Context, organizationID string) ([]Record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("device repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,vendor,model,serial_number,enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port FROM devices WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort); err != nil {
			return nil, err
		}
		d.SNMPConfigured = d.SNMPEnabled
		items = append(items, d)
	}
	return items, rows.Err()
}

// ListAllEnabled returns every enabled device across every organization,
// for background jobs (e.g. the metric-history sampler) that operate
// system-wide rather than scoped to one tenant's request.
func (r Repository) ListAllEnabled(ctx context.Context) ([]Record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("device repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,vendor,model,serial_number,enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port FROM devices WHERE enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort); err != nil {
			return nil, err
		}
		d.SNMPConfigured = d.SNMPEnabled
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r Repository) UpdateSNMP(ctx context.Context, id string, req SNMPConfigRequest) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET snmp_enabled=$2,snmp_version=$3,snmp_community=$4,snmp_username=$5,snmp_auth_protocol=$6,snmp_auth_password=$7,snmp_priv_protocol=$8,snmp_priv_password=$9,snmp_port=$10,snmp_timeout_ms=$11,updated_at=NOW() WHERE id=$1`, id, req.Enabled, req.Version, req.Community, req.Username, req.AuthProto, req.AuthPass, req.PrivProto, req.PrivPass, req.Port, req.TimeoutMS)
	return err
}
