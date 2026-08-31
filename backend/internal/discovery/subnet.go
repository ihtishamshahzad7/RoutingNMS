// Package discovery implements subnet auto-discovery: scan a CIDR range
// concurrently over SNMP, classify what responds, and hand back a list an
// operator can one-click add as monitored devices -- modeled on the
// auto-discovery flow described in NMS-Tool's README ("Scan any subnet,
// live progress, auto-classifies device types, select and add discovered
// devices in one click") but implemented natively against this project's
// existing snmp.Collector and devices.Repository.
package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// MaxHosts caps how many addresses a single scan will enumerate, so a
// mistyped "/8" can't turn into a multi-million-host scan.
const MaxHosts = 1024

// ExpandCIDR returns every usable host address in cidr (excluding the
// network and broadcast addresses for IPv4), erroring if the range is
// larger than MaxHosts (a /22 for IPv4).
func ExpandCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	if ip.To4() == nil {
		return nil, fmt.Errorf("only IPv4 CIDRs are supported")
	}

	var hosts []string
	for cur := cloneIP(ipnet.IP); ipnet.Contains(cur); incIP(cur) {
		hosts = append(hosts, cur.String())
		if len(hosts) > MaxHosts+2 { // +2 headroom before erroring, to give an exact message below
			return nil, fmt.Errorf("subnet is too large (max %d hosts; use a smaller CIDR like /22 or narrower)", MaxHosts)
		}
	}
	// Drop network and broadcast addresses for anything narrower than a /31.
	if len(hosts) > 2 {
		hosts = hosts[1 : len(hosts)-1]
	}
	if len(hosts) > MaxHosts {
		return nil, fmt.Errorf("subnet is too large (%d hosts; max %d -- use a smaller CIDR like /22 or narrower)", len(hosts), MaxHosts)
	}
	return hosts, nil
}

func cloneIP(ip net.IP) net.IP {
	out := make(net.IP, len(ip))
	copy(out, ip)
	return out
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
}

// Found is one responsive host discovered by a scan.
type Found struct {
	Address    string `json:"address"`
	SystemName string `json:"systemName,omitempty"`
	SysDescr   string `json:"sysDescr,omitempty"`
	SysObject  string `json:"sysObjectId,omitempty"`
	DeviceType string `json:"deviceType"` // router|switch|olt|server|other -- matches the devices table's CHECK constraint
	Vendor     string `json:"vendor,omitempty"`
}

// Classify guesses a devices.device_type-compatible bucket and vendor from
// sysDescr/sysObjectID text. It's a best-effort keyword heuristic (the same
// approach NMS-Tool and similar tools use for this) -- operators can always
// correct the type before importing.
func Classify(sysDescr, sysObjectID string) (deviceType, vendor string) {
	d := strings.ToLower(sysDescr)
	o := strings.ToLower(sysObjectID)
	text := d + " " + o

	switch {
	case strings.Contains(text, "mikrotik") || strings.Contains(text, "routeros"):
		vendor = "MikroTik"
	case strings.Contains(text, "cisco"):
		vendor = "Cisco"
	case strings.Contains(text, "huawei"):
		vendor = "Huawei"
	case strings.Contains(text, "zte"):
		vendor = "ZTE"
	case strings.Contains(text, "ubiquiti") || strings.Contains(text, "ubnt") || strings.Contains(text, "airmax") || strings.Contains(text, "unifi"):
		vendor = "Ubiquiti"
	case strings.Contains(text, "juniper"):
		vendor = "Juniper"
	case strings.Contains(text, "hp") || strings.Contains(text, "hewlett"):
		vendor = "HP"
	}

	switch {
	case strings.Contains(text, "olt") || strings.Contains(text, "gpon") || strings.Contains(text, "epon"):
		return "olt", vendor
	case strings.Contains(text, "printer") || strings.Contains(text, "laserjet") || strings.Contains(text, "deskjet"):
		return "other", vendor
	case strings.Contains(text, "switch"):
		return "switch", vendor
	case strings.Contains(text, "router") || strings.Contains(text, "routeros") || strings.Contains(text, "gateway"):
		return "router", vendor
	case strings.Contains(text, "linux") || strings.Contains(text, "windows") || strings.Contains(text, "server"):
		return "server", vendor
	default:
		return "other", vendor
	}
}

// ProbeOne does a single SNMP identity fetch (sysName/sysDescr/sysObjectID)
// against address using creds. It returns ok=false (not an error) for a
// plain timeout/no-response, since "nothing there" is the expected outcome
// for most of a subnet scan, not a failure worth surfacing per-host.
func ProbeOne(ctx context.Context, collector snmp.Collector, address string, port uint16, creds snmp.Credentials, timeoutMS int) (Found, bool) {
	target := snmp.Target{
		ID:          address,
		Address:     address,
		Port:        port,
		Credentials: creds,
		Timeout:     msToDuration(timeoutMS),
		Retries:     0,
	}
	values, err := collector.Get(ctx, target, []string{snmp.SysNameOID, snmp.SysDescrOID, snmp.SysObjectIDOID})
	if err != nil || len(values) == 0 {
		return Found{}, false
	}
	found := Found{Address: address}
	for _, v := range values {
		switch v.OID {
		case snmp.SysNameOID:
			found.SystemName = fmt.Sprint(v.Value)
		case snmp.SysDescrOID:
			found.SysDescr = fmt.Sprint(v.Value)
		case snmp.SysObjectIDOID:
			found.SysObject = fmt.Sprint(v.Value)
		}
	}
	found.DeviceType, found.Vendor = Classify(found.SysDescr, found.SysObject)
	return found, true
}

func msToDuration(ms int) time.Duration {
	if ms <= 0 {
		return 1500 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}
