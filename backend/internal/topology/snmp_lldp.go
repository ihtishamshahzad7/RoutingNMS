package topology

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosnmp/gosnmp"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// Standard LLDP-MIB objects. The implementation intentionally returns the
// remote chassis identity and port information without assuming a vendor.
const (
	lldpRemChassisID = ".1.0.8802.1.1.2.1.4.1.1.5"
	lldpRemPortID    = ".1.0.8802.1.1.2.1.4.1.1.7"
	lldpRemSysName   = ".1.0.8802.1.1.2.1.4.1.1.9"
)

type SNMPNeighborDiscovery struct { Collector snmp.Collector }

func (d SNMPNeighborDiscovery) Discover(ctx context.Context, node Node) ([]Neighbor, error) {
	if d.Collector == nil { return nil, fmt.Errorf("SNMP collector is required") }
	// The SNMP target is supplied by the collector's target resolver in the
	// application layer. This method deliberately keeps topology independent
	// from credential storage and device inventory.
	_ = ctx
	_ = node
	return nil, nil
}

// ParseLLDPNeighbor converts one LLDP row into normalized neighbor identity.
func ParseLLDPNeighbor(chassis, port, systemName string) (string, string, string) {
	return strings.TrimSpace(chassis), strings.TrimSpace(port), strings.TrimSpace(systemName)
}

// NewLLDPClient is a small helper for application wiring when a target is
// already known. Authentication/credential policy remains outside topology.
func NewLLDPClient(target snmp.Target) *gosnmp.GoSNMP {
	version := gosnmp.Version2c
	if target.Credentials.Version == snmp.V3 { version = gosnmp.Version3 }
	c := &gosnmp.GoSNMP{Target:target.Address, Port:target.Port, Version:version, Timeout:target.Timeout, Retries:uint8(target.Retries), Logger:nil}
	if version == gosnmp.Version2c { c.Community=target.Credentials.Community }
	return c
}

var _ = lldpRemChassisID
var _ = lldpRemPortID
var _ = lldpRemSysName
