package devices

import (
	"context"
	"fmt"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

func (r Repository) DiscoveryTarget(ctx context.Context, id string) (DeviceInput, error) {
	if r.DB == nil { return DeviceInput{}, fmt.Errorf("device repository is not initialized") }
	var in DeviceInput
	var version string
	var timeoutMS int
	var enabled bool
	err := r.DB.QueryRow(ctx, `SELECT organization_id,name,address,device_type,vendor,snmp_enabled,snmp_version,COALESCE(snmp_community,''),COALESCE(snmp_username,''),COALESCE(snmp_auth_protocol,''),COALESCE(snmp_auth_password,''),COALESCE(snmp_priv_protocol,''),COALESCE(snmp_priv_password,''),COALESCE(snmp_port,161),COALESCE(snmp_timeout_ms,3000) FROM devices WHERE id=$1`, id).Scan(&in.OrganizationID,&in.Name,&in.Address,&in.DeviceType,&in.Vendor,&enabled,&version,&in.SNMP.Community,&in.SNMP.Username,&in.SNMP.AuthProto,&in.SNMP.AuthPass,&in.SNMP.PrivProto,&in.SNMP.PrivPass,&in.SNMPPort,&timeoutMS)
	if err != nil { return DeviceInput{}, fmt.Errorf("load device: %w", err) }
	if !enabled { return DeviceInput{}, fmt.Errorf("SNMP monitoring is disabled for this device") }
	in.SNMP.Version = snmp.Version(version)
	in.Timeout = time.Duration(timeoutMS) * time.Millisecond
	return in, nil
}
