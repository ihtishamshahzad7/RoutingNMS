package topology

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

const (
	lldpRemChassisID = ".1.0.8802.1.1.2.1.4.1.1.5"
	lldpRemPortID    = ".1.0.8802.1.1.2.1.4.1.1.7"
	lldpRemSysName   = ".1.0.8802.1.1.2.1.4.1.1.9"
)

type TargetResolver func(node Node) (snmp.Target, error)

type SNMPNeighborDiscovery struct {
	Collector snmp.Collector
	Resolve TargetResolver
}

func (d SNMPNeighborDiscovery) Discover(ctx context.Context, node Node) ([]Neighbor, error) {
	if strings.TrimSpace(node.Address) == "" { return nil, fmt.Errorf("node %s has no SNMP address", node.ID) }
	if d.Resolve == nil { return nil, fmt.Errorf("SNMP target resolver is required") }
	target, err := d.Resolve(node); if err != nil { return nil, err }
	chassis, err := d.Collector.Walk(ctx, target, lldpRemChassisID); if err != nil { return nil, err }
	ports, err := d.Collector.Walk(ctx, target, lldpRemPortID); if err != nil { return nil, err }
	systems, err := d.Collector.Walk(ctx, target, lldpRemSysName); if err != nil { return nil, err }

	byRow := map[string]*Neighbor{}
	for _, v := range chassis { row := lldpRow(v.OID, lldpRemChassisID); if row == "" { continue }; n:=byRow[row]; if n==nil { n=&Neighbor{LocalID:node.ID,Status:Up}; byRow[row]=n }; n.RemoteID=stringValue(v.Value) }
	for _, v := range ports { row:=lldpRow(v.OID,lldpRemPortID); if row=="" {continue}; n:=byRow[row];if n==nil{n=&Neighbor{LocalID:node.ID,Status:Up};byRow[row]=n}; _=stringValue(v.Value) }
	for _, v := range systems { row:=lldpRow(v.OID,lldpRemSysName);if row==""{continue};n:=byRow[row];if n==nil{n=&Neighbor{LocalID:node.ID,Status:Up};byRow[row]=n};if name:=stringValue(v.Value);name!=""{n.RemoteID=name} }
	out:=make([]Neighbor,0,len(byRow));for _,n:=range byRow{if strings.TrimSpace(n.RemoteID)!=""{out=append(out,*n)}}
	return out,nil
}

func lldpRow(oid, base string) string { oid=strings.TrimPrefix(oid,".");base=strings.TrimPrefix(base,".");if !strings.HasPrefix(oid,base+"."){return ""};return strings.TrimPrefix(oid,base+".") }

func stringValue(v any) string { switch x:=v.(type){case string:return strings.TrimSpace(x);case []byte:return strings.TrimSpace(string(x));case fmt.Stringer:return strings.TrimSpace(x.String());default:return strings.TrimSpace(strconv.FormatBool(false))} }

func ParseLLDPNeighbor(chassis, port, systemName string) (string,string,string) { return strings.TrimSpace(chassis),strings.TrimSpace(port),strings.TrimSpace(systemName) }
