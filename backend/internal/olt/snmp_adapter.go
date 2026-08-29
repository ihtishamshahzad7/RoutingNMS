package olt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type SNMPAdapter struct { Target snmp.Target; Mapping OIDMapping; Collector snmp.Collector }
type OIDMapping struct { PONName, PONStatus, ONUSerial, ONUStatus, ONULOS, ONURXPower, ONUTXPower string }

func (a *SNMPAdapter) Discover(ctx context.Context, olt OLT) ([]PONPort, error) {
	if a.Mapping.PONName == "" { return nil, fmt.Errorf("PON OID mapping is not configured") }
	values, err := a.Collector.Walk(ctx, a.targetFor(olt), a.Mapping.PONName); if err != nil { return nil, fmt.Errorf("walk PON names: %w", err) }
	pons := make([]PONPort, 0, len(values))
	for i, v := range values { pons = append(pons, PONPort{ID:v.OID, OLTID:olt.ID, Name:fmt.Sprint(v.Value), Index:i+1, Type:"pon", Status:Unknown}) }
	return pons,nil
}

func (a *SNMPAdapter) DiscoverONUs(ctx context.Context, olt OLT, port PONPort) ([]ONU,error) {
	if a.Mapping.ONUSerial=="" { return nil,fmt.Errorf("ONU serial OID mapping is not configured") }
	values,err:=a.Collector.Walk(ctx,a.targetFor(olt),a.Mapping.ONUSerial);if err!=nil{return nil,fmt.Errorf("walk ONU serials on %s: %w",port.Name,err)}
	onus:=make([]ONU,0)
	for _,v:=range values {
		// Do not guess a vendor-specific compound index. The ONU table index is
		// preserved in the ID; a vendor profile can provide exact parent mapping.
		onus=append(onus,ONU{ID:v.OID,OLTID:olt.ID,PONPortID:port.ID,Serial:strings.TrimSpace(fmt.Sprint(v.Value)),Status:Unknown})
	}
	return onus,nil
}

func (a *SNMPAdapter) PollONU(ctx context.Context, olt OLT, onu ONU)(ONU,error){
	select{case <-ctx.Done():return onu,ctx.Err();default:}
	// Metric OIDs are table/index dependent. The mapping is expected to contain
	// an exact indexed OID template in production; avoid fabricating indexes.
	return onu,nil
}

func (a *SNMPAdapter) targetFor(olt OLT) snmp.Target { t:=a.Target;if strings.TrimSpace(olt.Address)!=""{t.Address=olt.Address};return t }

func (a *SNMPAdapter) pollIndexed(ctx context.Context, olt OLT, onu ONU, oids []string) ([]snmp.Value,error) {
	if len(oids)==0{return nil,nil}; return a.Collector.Get(ctx,a.targetFor(olt),oids)
}

func (a *SNMPAdapter) markSeen(onu *ONU) { now:=time.Now().UTC(); onu.LastSeen=&now }

var _ Adapter = (*SNMPAdapter)(nil)
