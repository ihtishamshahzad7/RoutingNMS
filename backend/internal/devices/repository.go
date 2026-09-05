package devices

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// generatePushToken creates a 32-hex-character random token for the push
// heartbeat monitor URL -- long and random enough that guessing/enumerating
// a valid token is not a practical concern, matching Kuma's own push
// monitor token scheme.
func generatePushToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

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
	DNSEnabled                bool       `json:"dnsEnabled"`
	DNSHostname               string     `json:"dnsHostname,omitempty"`
	DNSRecordType             string     `json:"dnsRecordType"`
	DNSResolverServer         string     `json:"dnsResolverServer,omitempty"`
	DNSExpectedAnswer         string     `json:"dnsExpectedAnswer,omitempty"`
	DNSIntervalSeconds        int        `json:"dnsIntervalSeconds"`
	PushEnabled               bool       `json:"pushEnabled"`
	PushToken                 string     `json:"pushToken,omitempty"`
	PushIntervalSeconds       int        `json:"pushIntervalSeconds"`
	PushGracePeriodSeconds    int        `json:"pushGracePeriodSeconds"`
	PushLastSeenAt            *time.Time `json:"pushLastSeenAt,omitempty"`
	PushLastStatus            string     `json:"pushLastStatus,omitempty"`
	PushLastMessage           string     `json:"pushLastMessage,omitempty"`
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

// DNSCheckRequest configures the optional DNS resolution monitor on a
// device -- ported from Uptime Kuma's "DNS" monitor type: resolve a
// hostname as a given record type, optionally against a specific resolver
// server, optionally verifying the answer matches an expected value.
type DNSCheckRequest struct {
	Enabled         bool   `json:"enabled"`
	Hostname        string `json:"hostname"`
	RecordType      string `json:"recordType"`
	ResolverServer  string `json:"resolverServer"`
	ExpectedAnswer  string `json:"expectedAnswer"`
	IntervalSeconds int    `json:"intervalSeconds"`
}

// PushCheckRequest configures the optional "push" heartbeat monitor on a
// device -- ported from Uptime Kuma's "Push" monitor type: the monitored
// thing calls RoutingNMS on its own schedule instead of being polled.
// Enabling doesn't itself generate the token -- UpdatePushCheck generates
// one the first time Enabled=true and no token exists yet.
type PushCheckRequest struct {
	Enabled            bool `json:"enabled"`
	IntervalSeconds    int  `json:"intervalSeconds"`
	GracePeriodSeconds int  `json:"gracePeriodSeconds"`
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
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,vendor,model,serial_number,enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,http_check_enabled,http_url,http_expected_status,http_keyword,http_timeout_ms,icmp_enabled,icmp_interval_seconds,icmp_packet_size,icmp_count,icmp_retries,dns_enabled,dns_hostname,dns_record_type,dns_resolver_server,dns_expected_answer,dns_interval_seconds,push_enabled,COALESCE(push_token,''),push_interval_seconds,push_grace_period_seconds,push_last_seen_at,push_last_status,push_last_message FROM devices WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.HTTPCheckEnabled, &d.HTTPURL, &d.HTTPExpectedStatus, &d.HTTPKeyword, &d.HTTPTimeoutMS, &d.ICMPEnabled, &d.ICMPIntervalSeconds, &d.ICMPPacketSize, &d.ICMPCount, &d.ICMPRetries, &d.DNSEnabled, &d.DNSHostname, &d.DNSRecordType, &d.DNSResolverServer, &d.DNSExpectedAnswer, &d.DNSIntervalSeconds, &d.PushEnabled, &d.PushToken, &d.PushIntervalSeconds, &d.PushGracePeriodSeconds, &d.PushLastSeenAt, &d.PushLastStatus, &d.PushLastMessage); err != nil {
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
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,vendor,model,serial_number,enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,http_check_enabled,http_url,http_expected_status,http_keyword,http_timeout_ms,icmp_enabled,icmp_interval_seconds,icmp_packet_size,icmp_count,icmp_retries,dns_enabled,dns_hostname,dns_record_type,dns_resolver_server,dns_expected_answer,dns_interval_seconds,push_enabled,COALESCE(push_token,''),push_interval_seconds,push_grace_period_seconds,push_last_seen_at,push_last_status,push_last_message FROM devices WHERE enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.HTTPCheckEnabled, &d.HTTPURL, &d.HTTPExpectedStatus, &d.HTTPKeyword, &d.HTTPTimeoutMS, &d.ICMPEnabled, &d.ICMPIntervalSeconds, &d.ICMPPacketSize, &d.ICMPCount, &d.ICMPRetries, &d.DNSEnabled, &d.DNSHostname, &d.DNSRecordType, &d.DNSResolverServer, &d.DNSExpectedAnswer, &d.DNSIntervalSeconds, &d.PushEnabled, &d.PushToken, &d.PushIntervalSeconds, &d.PushGracePeriodSeconds, &d.PushLastSeenAt, &d.PushLastStatus, &d.PushLastMessage); err != nil {
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

// UpdateDNSCheck configures (or disables) the optional DNS resolution
// monitor on a device.
func (r Repository) UpdateDNSCheck(ctx context.Context, id string, req DNSCheckRequest) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	if req.RecordType == "" {
		req.RecordType = "A"
	}
	if req.IntervalSeconds < 5 {
		req.IntervalSeconds = 60
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET dns_enabled=$2,dns_hostname=$3,dns_record_type=$4,dns_resolver_server=$5,dns_expected_answer=$6,dns_interval_seconds=$7,updated_at=NOW() WHERE id=$1`,
		id, req.Enabled, req.Hostname, req.RecordType, req.ResolverServer, req.ExpectedAnswer, req.IntervalSeconds)
	return err
}

// UpdatePushCheck configures the optional "push" heartbeat monitor on a
// device. If enabling for the first time (no push_token stored yet), a new
// random token is generated so the caller can build the push URL. Returns
// the (possibly newly generated) token.
func (r Repository) UpdatePushCheck(ctx context.Context, id string, req PushCheckRequest) (string, error) {
	if r.DB == nil {
		return "", fmt.Errorf("device repository is not initialized")
	}
	if req.IntervalSeconds < 10 {
		req.IntervalSeconds = 60
	}
	if req.GracePeriodSeconds < 0 {
		req.GracePeriodSeconds = 30
	}
	var existingToken string
	if err := r.DB.QueryRow(ctx, `SELECT COALESCE(push_token,'') FROM devices WHERE id=$1`, id).Scan(&existingToken); err != nil {
		return "", err
	}
	token := existingToken
	if req.Enabled && token == "" {
		generated, err := generatePushToken()
		if err != nil {
			return "", err
		}
		token = generated
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET push_enabled=$2,push_interval_seconds=$3,push_grace_period_seconds=$4,push_token=$5,updated_at=NOW() WHERE id=$1`,
		id, req.Enabled, req.IntervalSeconds, req.GracePeriodSeconds, token)
	if err != nil {
		return "", err
	}
	return token, nil
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
	err := r.DB.QueryRow(ctx, `SELECT id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds,snmp_enabled,snmp_version,snmp_port,provisioning_template_id,last_provisioned_at,http_check_enabled,http_url,http_expected_status,http_keyword,http_timeout_ms,icmp_enabled,icmp_interval_seconds,icmp_packet_size,icmp_count,icmp_retries,dns_enabled,dns_hostname,dns_record_type,dns_resolver_server,dns_expected_answer,dns_interval_seconds,push_enabled,COALESCE(push_token,''),push_interval_seconds,push_grace_period_seconds,push_last_seen_at,push_last_status,push_last_message FROM devices WHERE id=$1`, id).
		Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.SNMPEnabled, &d.SNMPVersion, &d.SNMPPort, &d.ProvisioningTemplateID, &d.LastProvisionedAt, &d.HTTPCheckEnabled, &d.HTTPURL, &d.HTTPExpectedStatus, &d.HTTPKeyword, &d.HTTPTimeoutMS, &d.ICMPEnabled, &d.ICMPIntervalSeconds, &d.ICMPPacketSize, &d.ICMPCount, &d.ICMPRetries, &d.DNSEnabled, &d.DNSHostname, &d.DNSRecordType, &d.DNSResolverServer, &d.DNSExpectedAnswer, &d.DNSIntervalSeconds, &d.PushEnabled, &d.PushToken, &d.PushIntervalSeconds, &d.PushGracePeriodSeconds, &d.PushLastSeenAt, &d.PushLastStatus, &d.PushLastMessage)
	d.SNMPConfigured = d.SNMPEnabled
	return d, err
}

// GetByPushToken looks up an enabled, push-monitor-enabled device by its
// push token -- used by the unauthenticated push-receive endpoint (external
// cron jobs/services call this URL with no session, like RouterOS
// provisioning fetch).
func (r Repository) GetByPushToken(ctx context.Context, token string) (Record, error) {
	if r.DB == nil {
		return Record{}, fmt.Errorf("device repository is not initialized")
	}
	var d Record
	err := r.DB.QueryRow(ctx, `SELECT id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds,push_interval_seconds,push_grace_period_seconds FROM devices WHERE push_token=$1 AND push_enabled=true AND enabled=true`, token).
		Scan(&d.ID, &d.OrganizationID, &d.Name, &d.Address, &d.DeviceType, &d.Vendor, &d.Model, &d.SerialNumber, &d.Enabled, &d.MonitoringIntervalSeconds, &d.PushIntervalSeconds, &d.PushGracePeriodSeconds)
	return d, err
}

// RecordPush stores the arrival of a heartbeat push -- status/msg as
// reported by the caller, and the current timestamp as push_last_seen_at
// (what the down-detection sweep compares against interval+grace).
func (r Repository) RecordPush(ctx context.Context, id string, status, message string) error {
	if r.DB == nil {
		return fmt.Errorf("device repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `UPDATE devices SET push_last_seen_at=NOW(),push_last_status=$2,push_last_message=$3,updated_at=NOW() WHERE id=$1`,
		id, status, message)
	return err
}

// ListPushEnabled returns every enabled device with push_enabled=true, for
// the down-detection sweep.
func (r Repository) ListPushEnabled(ctx context.Context) ([]Record, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("device repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,name,push_interval_seconds,push_grace_period_seconds,push_last_seen_at FROM devices WHERE enabled=true AND push_enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.Name, &d.PushIntervalSeconds, &d.PushGracePeriodSeconds, &d.PushLastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
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
