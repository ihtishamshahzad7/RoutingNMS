package topology

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

const lldpRemLocalPort = ".1.0.8802.1.1.2.1.4.1.1.2"

// ParseLLDPTable joins LLDP-MIB columns by their row index. This preserves the
// local interface as well as the remote interface so topology links are useful
// to NOC operators and not merely device-to-device edges.
func ParseLLDPTable(values []snmp.Value) ([]Relationship, error) {
	type row struct { local, chassis, remotePort, system string }
	rows := map[string]*row{}
	for _, v := range values {
		parts := strings.Split(strings.TrimPrefix(v.OID, "."), ".")
		if len(parts) < 1 { continue }
		idx := parts[len(parts)-1]
		r := rows[idx]; if r == nil { r=&row{}; rows[idx]=r }
		s := fmt.Sprint(v.Value)
		switch {
		case strings.HasPrefix(v.OID, lldpRemLocalPort): r.local=s
		case strings.HasPrefix(v.OID, lldpRemChassisID): r.chassis=s
		case strings.HasPrefix(v.OID, lldpRemPortID): r.remotePort=s
		case strings.HasPrefix(v.OID, lldpRemSysName): r.system=s
		}
	}
	out:=make([]Relationship,0,len(rows))
	for idx,r:=range rows { if r.local==""||r.chassis=="" { continue }; remote:=r.system; if remote=="" {remote=r.chassis}; out=append(out,Relationship{SourceID:r.local,TargetID:remote,Status:Up}); _=r.remotePort; _=idx }
	return out,nil
}

func decodeSNMPIndex(value any) string { if b,ok:=value.([]byte); ok { return string(b) }; return strconv.FormatInt(0,10) }
