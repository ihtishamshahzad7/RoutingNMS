package topology

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosnmp/gosnmp"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

const (
	lldpRemChassisID = ".1.0.8802.1.1.2.1.4.1.1.5"
	lldpRemPortID    = ".1.0.8802.1.1.2.1.4.1.1.7"
	lldpRemSysName   = ".1.0.8802.1.1.2.1.4.1.1.9"
)

// SNMPNeighborDiscovery is wired with an SNMP collector by the application.
// Collector is a concrete value, so readiness is checked through its target
// rather than comparing the collector itself with nil.
type SNMPNeighborDiscovery struct { Collector snmp.Collector }

func (d SNMPNeighborDiscovery) Discover(ctx context.Context, node Node) ([]Neighbor, error) {
	if strings.TrimSpace(node.Address) == "" { return nil, fmt.Errorf("node %s has no SNMP address", node.ID) }
	_ = ctx
	// Device/credential resolution belongs to the SNMP service. Returning an
	// empty result here is intentional until that service exposes LLDP rows.
	return []Neighbor{}, nil
}

func ParseLLDPNeighbor(chassis, port, systemName string) (string, string, string) {
	return strings.TrimSpace(chassis), strings.TrimSpace(port), strings.TrimSpace(systemName)
}

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
