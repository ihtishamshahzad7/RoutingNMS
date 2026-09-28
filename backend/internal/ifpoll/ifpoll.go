// Package ifpoll implements periodic SNMP interface (IF-MIB) polling for
// every SNMP-enabled device (item 3.1 of the SNMP & Syslog Monitoring
// build): current per-port state plus an up/down transition history, and
// device-level SNMP reachability tracked separately from per-port status
// so a device that stops answering SNMP is never reported as having its
// ports "down" -- unknown is not the same as down.
//
// This reuses internal/snmp's existing Collector/Target/Credentials (the
// same client and credential handling the manual "Discover" button
// already uses via internal/devices.DiscoveryTarget) and
// internal/pollpool's bounded worker pool, and follows the same
// Repository/Poller shape as internal/ping and internal/portcheck so it
// runs through the existing poller pattern rather than a parallel one.
package ifpoll

import (
	"strconv"
	"strings"

	gosnmp "github.com/gosnmp/gosnmp"
)

// PortState is one interface's polled state for one cycle.
type PortState struct {
	IfIndex      int64
	Description  string
	AdminUp      bool
	OperUp       bool
	SpeedBps     uint64
	InOctets     uint64
	OutOctets    uint64
	CounterWidth int // 32 or 64 -- which counter family InOctets/OutOctets came from
}

func uint64Value(v any) uint64 {
	switch x := v.(type) {
	case uint64:
		return x
	case uint32:
		return uint64(x)
	case int:
		return uint64(x)
	case int64:
		if x < 0 {
			return 0
		}
		return uint64(x)
	case string:
		n, _ := strconv.ParseUint(strings.TrimSpace(x), 10, 64)
		return n
	case []byte:
		var n uint64
		for _, b := range x {
			n = n<<8 | uint64(b)
		}
		return n
	default:
		return 0
	}
}

// walkOID walks oid on client, calling apply(ifIndex, value) for each
// returned varbind. Rows whose trailing OID segment doesn't parse as an
// integer index are skipped rather than aborting the whole walk.
func walkOID(client *gosnmp.GoSNMP, oid string, apply func(idx int64, v any)) error {
	return client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		idxStr := strings.TrimPrefix(pdu.Name, oid+".")
		idxStr = strings.TrimPrefix(idxStr, ".")
		idx, err := strconv.ParseInt(idxStr, 10, 64)
		if err != nil {
			return nil
		}
		apply(idx, pdu.Value)
		return nil
	})
}
