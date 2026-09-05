package devices

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

type Record struct {
	ID                        string     `json:"id"`
	OrganizationID            string     `json:"organizationId"`
	Name                      string     `json:"name"`
	Address                   string     `json:"address"`
	DeviceType                string     `json:"deviceType"`
	Vendor                    string     `json:"vendor,omitempty"`
	Model                     string     `json:"model,omitempty"`
	SerialNumber              string     `json:"serialNumber,omitempty"`
	Enabled                   bool       `json:"enabled"`
	MonitoringIntervalSeconds int        `json:"monitoringIntervalSeconds"`
	SNMPEnabled               bool       `json:"snmpEnabled"`
	SNMPVersion               string     `json:"snmpVersion"`
	SNMPPort                  int        `json:"snmpPort"`
	SNMPConfigured            bool       `json:"snmpConfigured"`
	ProvisioningTemplateID    *int64     `json:"provisioningTemplateId,omitempty"`
	LastProvisionedAt         *time.Time `json:"lastProvisionedAt,omitempty"`
	HTTPCheckEnabled          bool       `json:"httpCheckEnabled"`
	HTTPURL                   string     `json:"httpUrl,omitempty"`
	HTTPExpectedStatus        int        `json:"httpExpectedStatus"`
	HTTPKeyword               string     `json:"httpKeyword,omitempty"`
	HTTPTimeoutMS             int        `json:"httpTimeoutMs"`
	ICMPEnabled               bool       `json:"icmpEnabled"`
	ICMPIntervalSeconds       int        `json:"icmpIntervalSeconds"`
	ICMPPacketSize            int        `json:"icmpPacketSize"`
	ICMPCount                 int        `json:"icmpCount"`
	ICMPRetries               int        `json:"icmpRetries"`
}

// ICMPCheckRequest configures the dedicated ICMP ping poller (internal/ping)
// on a device -- interval/packet size/probe count/retries, mirroring the
// per-monitor options Uptime Kuma's ping monitor exposes.
type ICMPCheckRequest struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"intervalSeconds"`
	PacketSize      int  `json:"packetSize"`
	Count           int  `json:"count"`
	Retries         int  `json:"retries"`
}

// HTTPCheckRequest configures the optional HTTP(S)+keyword monitor on a
// device -- ported from Uptime Kuma's "http"/"keyword" monitor types. A
// device can have this enabled alongside SNMP/ICMP monitoring.
type HTTPCheckRequest struct {
	Enabled        bool   `json:"enabled"`
	URL            string `json:"url"`
	ExpectedStatus int    `json:"expectedStatus"`
	Keyword        string `json:"keyword"`
	TimeoutMS      int    `json:"timeoutMs"`
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
	err := r.DB.QueryRow(ctx, `INSERT INTO devices (organization_id,name,address,device_type,vendor,serial_number,enabled,snmp_enabled,snmp_version,snmp_community,snmp_username,snmp_auth_protocol,snmp_auth_password,snmp_priv_protocol,snmp_priv_password,snmp_port,snmp_timeout_ms) VALUES ($1,$2,$3,$4,$5,$6,true,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port`, in.OrganizationID, in.Name, in.Address, in.DeviceType, in.Vendor, in.SerialNumber, in.SNMP.Version != "", in.SNMP.Version, in.SNMP.Community, in.SNMP.Username, in.SNMP.AuthProto, in.SNMP.AuthPass, in.SNMP.PrivProto, in.SNMP.PrivPass, in.SNMPPort, int(in.Timeout/time.Millisecond)).Scan(&out.ID, &out.OrganizationID, &out.Name, &out.Address, &out.DeviceType, &out.Vendor, &out.Model, &out.SerialNumber, &out.Enabled, &out.MonitoringIntervalSeconds, &out.SNMPEnabled, &out.SNMPVersion, &out.SNMPPort)
	out.SNMPConfigured = out.SNMPEnabled
	return out, err
}

func (r Repository) List(ctx context.Context, organizationID string) ([]Record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("device repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,vendor,model,serial_number,enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,http_check_enabled,http_url,http_expected_status,http_keyword,http_timeout_ms,icmp_enabled,icmp_interval_seconds,icmp_packet_size,icmp_count,icmp_retries FROM devices WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.HTTPCheckEnabled, &d.HTTPURL, &d.HTTPExpectedStatus, &d.HTTPKeyword, &d.HTTPTimeoutMS, &d.ICMPEnabled, &d.ICMPIntervalSeconds, &d.ICMPPacketSize, &d.ICMPCount, &d.ICMPRetries); err != nil {
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
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,vendor,model,serial_number,enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,http_check_enabled,http_url,http_expected_status,http_keyword,http_timeout_ms,icmp_enabled,icmp_interval_seconds,icmp_packet_size,icmp_count,icmp_retries FROM devices WHERE enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.HTTPCheckEnabled, &d.HTTPURL, &d.HTTPExpectedStatus, &d.HTTPKeyword, &d.HTTPTimeoutMS, &d.ICMPEnabled, &d.ICMPIntervalSeconds, &d.ICMPPacketSize, &d.ICMPCount, &d.ICMPRetries); err != nil {
			return nil, err
		}
		d.SNMPConfigured = d.SNMPEnabled
		items = append(items, d)
	}
	return items, rows.Err()
}

// UpdateHTTPCheck configures (or disables) the optional HTTP(S)+keyword
// monitor on a device.
func (r Repository) UpdateHTTPCheck(ctx context.Context, id string, req HTTPCheckRequest) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	if req.ExpectedStatus == 0 {
		req.ExpectedStatus = 200
	}
	if req.TimeoutMS <= 0 {
		req.TimeoutMS = 5000
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET http_check_enabled=$2,http_url=$3,http_expected_status=$4,http_keyword=$5,http_timeout_ms=$6,updated_at=NOW() WHERE id=$1`,
		id, req.Enabled, req.URL, req.ExpectedStatus, req.Keyword, req.TimeoutMS)
	return err
}

// UpdateICMPCheck configures the dedicated ICMP ping poller for a device --
// interval/packet size/count/retries. Retries=0 is coerced up to 1 (fire
// immediately) rather than treated as "never down".
func (r Repository) UpdateICMPCheck(ctx context.Context, id string, req ICMPCheckRequest) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	if req.IntervalSeconds < 5 {
		req.IntervalSeconds = 30
	}
	if req.PacketSize <= 0 {
		req.PacketSize = 56
	}
	if req.Count <= 0 {
		req.Count = 3
	}
	if req.Retries <= 0 {
		req.Retries = 1
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET icmp_enabled=$2,icmp_interval_seconds=$3,icmp_packet_size=$4,icmp_count=$5,icmp_retries=$6,updated_at=NOW() WHERE id=$1`,
		id, req.Enabled, req.IntervalSeconds, req.PacketSize, req.Count, req.Retries)
	return err
}

func (r Repository) UpdateSNMP(ctx context.Context, id string, req SNMPConfigRequest) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET snmp_enabled=$2,snmp_version=$3,snmp_community=$4,snmp_username=$5,snmp_auth_protocol=$6,snmp_auth_password=$7,snmp_priv_protocol=$8,snmp_priv_password=$9,snmp_port=$10,snmp_timeout_ms=$11,updated_at=NOW() WHERE id=$1`, id, req.Enabled, req.Version, req.Community, req.Username, req.AuthProto, req.AuthPass, req.PrivProto, req.PrivPass, req.Port, req.TimeoutMS)
	return err
}

// GetBySerial looks up an enabled device by its serial number -- used by the
// RouterOS provisioning fetch endpoint, which authenticates the caller by a
// shared token rather than a session, so the serial number in the URL is the
// only way to identify which device is asking.
func (r Repository) GetBySerial(ctx context.Context, serial string) (Record, error) {
	if r.DB == nil {
		return Record{}, fmt.Errorf("device repository is not initialized")
	}
	var d Record
	err := r.DB.QueryRow(ctx, `SELECT id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,provisioning_template_id,last_provisioned_at FROM devices WHERE serial_number=$1 AND enabled=true`, serial).
		Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.ProvisioningTemplateID, &d.LastProvisionedAt)
	d.SNMPConfigured = d.SNMPEnabled
	return d, err
}

// GetByID looks up a single device by ID, for the provisioning preview endpoint.
func (r Repository) GetByID(ctx context.Context, id string) (Record, error) {
	if r.DB == nil {
		return Record{}, fmt.Errorf("device repository is not initialized")
	}
	var d Record
	err := r.DB.QueryRow(ctx, `SELECT id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,provisioning_template_id,last_provisioned_at,http_check_enabled,http_url,http_expected_status,http_keyword,http_timeout_ms,icmp_enabled,icmp_interval_seconds,icmp_packet_size,icmp_count,icmp_retries FROM devices WHERE id=$1`, id).
		Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.ProvisioningTemplateID, &d.LastProvisionedAt, &d.HTTPCheckEnabled, &d.HTTPURL, &d.HTTPExpectedStatus, &d.HTTPKeyword, &d.HTTPTimeoutMS, &d.ICMPEnabled, &d.ICMPIntervalSeconds, &d.ICMPPacketSize, &d.ICMPCount, &d.ICMPRetries)
	d.SNMPConfigured = d.SNMPEnabled
	return d, err
}

// UpdateProvisioning assigns (or clears, with a nil templateID) the
// provisioning template for a device.
func (r Repository) UpdateProvisioning(ctx context.Context, id string, templateID *int64) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET provisioning_template_id=$2, updated_at=NOW() WHERE id=$1`, id, templateID)
	return err
}

// TouchProvisioned records that a device successfully fetched its
// provisioning script just now.
func (r Repository) TouchProvisioned(ctx context.Context, id string) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET last_provisioned_at=NOW() WHERE id=$1`, id)
	return err
}
