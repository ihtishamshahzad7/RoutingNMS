package olt

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type SNMPAdapter struct { Target snmp.Target; Mapping OIDMapping; Collector snmp.Collector }
type OIDMapping struct {
	PONName, PONStatus, ONUSerial, ONUStatus, ONULOS, ONURXPower, ONUTXPower string
	ONUStatusOID, ONULOSOID, ONURXPowerOID, ONUTXPowerOID OIDTemplate
	// ONUIndex defines how the ONU serial table identifies its parent PON.
	// Positions are zero-based within the numeric suffix after ONUSerial.
	ONUIndex ONUIndexSpec
}

type ONUIndexSpec struct { PONPosition int; ONUPosition int }

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
		if !a.Mapping.ONUIndex.Valid() { return nil,fmt.Errorf("ONU index mapping is not configured") }
		parent,ok:=a.Mapping.ONUIndex.ParentIndex(v.OID,a.Mapping.ONUSerial);if !ok || parent!=port.Index { continue }
		onus=append(onus,ONU{ID:v.OID,OLTID:olt.ID,PONPortID:port.ID,Serial:strings.TrimSpace(fmt.Sprint(v.Value)),Status:Unknown})
	}
	return onus,nil
}

func (a *SNMPAdapter) PollONU(ctx context.Context, olt OLT, onu ONU)(ONU,error){
	select{case <-ctx.Done():return onu,ctx.Err();default:}
	index:=onuIndex(onu.ID,a.Mapping.ONUSerial);if index==""{return onu,fmt.Errorf("cannot derive ONU table index from %q",onu.ID)}
	type metric struct{template OIDTemplate;set func(snmp.Value)}
	metrics:=[]metric{{a.Mapping.ONUStatusOID,func(v snmp.Value){onu.Status=parseStatus(v.Value)}},{a.Mapping.ONULOSOID,func(v snmp.Value){onu.LOS=parseBool(v.Value)}},{a.Mapping.ONURXPowerOID,func(v snmp.Value){if n,ok:=parseFloat(v.Value);ok{onu.RxPowerDBm=&n}}},{a.Mapping.ONUTXPowerOID,func(v snmp.Value){if n,ok:=parseFloat(v.Value);ok{onu.TxPowerDBm=&n}}}}
	oids:=make([]string,0,len(metrics));active:=make([]metric,0,len(metrics))
	for _,m:=range metrics{if !m.template.Valid(){continue};oid,err:=m.template.Indexed(index);if err!=nil{return onu,err};oids=append(oids,oid);active=append(active,m)}
	if len(oids)==0{return onu,fmt.Errorf("no ONU metric OID templates configured")}
	values,err:=a.pollIndexed(ctx,olt,onu,oids);if err!=nil{return onu,err};for i,v:=range values{if i<len(active){active[i].set(v)}}
	now:=time.Now().UTC();onu.LastSeen=&now;return onu,nil
}

func (a *SNMPAdapter) targetFor(olt OLT) snmp.Target { t:=a.Target;if strings.TrimSpace(olt.Address)!=""{t.Address=olt.Address};return t }
func (a *SNMPAdapter) pollIndexed(ctx context.Context, olt OLT, onu ONU, oids []string)([]snmp.Value,error){return a.Collector.Get(ctx,a.targetFor(olt),oids)}
func onuIndex(id,base string)string{id=strings.Trim(id,".");base=strings.TrimRight(strings.Trim(base),".");if base!=""{prefix:=strings.Trim(base,".")+".";if strings.HasPrefix(id,prefix){return strings.TrimPrefix(id,prefix)}};parts:=strings.Split(id,".");if len(parts)==0{return ""};return parts[len(parts)-1]}
func parseFloat(v any)(float64,bool){switch x:=v.(type){case float64:return x,true;case float32:return float64(x),true;case int:return float64(x),true;case int64:return float64(x),true;case uint64:return float64(x),true;case string:n,e:=strconv.ParseFloat(strings.TrimSpace(x),64);return n,e==nil};return 0,false}
func parseBool(v any)bool{switch x:=v.(type){case bool:return x;case int:return x!=0;case int64:return x!=0;case uint64:return x!=0;case string:s:=strings.ToLower(strings.TrimSpace(x));return s=="1"||s=="true"||s=="yes"||s=="on"||s=="los"};return false}
func parseStatus(v any)Status{if n,ok:=parseFloat(v);ok{if n==1{return Online};if n==2||n==3||n==0{return Offline}};s:=strings.ToLower(strings.TrimSpace(fmt.Sprint(v)));if strings.Contains(s,"online")||strings.Contains(s,"up"){return Online};if strings.Contains(s,"offline")||strings.Contains(s,"down"){return Offline};return Unknown}

func (s ONUIndexSpec) Valid() bool { return s.PONPosition >= 0 && s.ONUPosition >= 0 && s.PONPosition != s.ONUPosition }
func (s ONUIndexSpec) ParentIndex(oid,base string)(int,bool){
	if !s.Valid(){return 0,false};suffix:=strings.TrimPrefix(strings.Trim(oid,"."),strings.TrimRight(strings.Trim(base,"."),".")+".");parts:=strings.Split(suffix,".");if s.PONPosition>=len(parts){return 0,false};n,e:=strconv.Atoi(parts[s.PONPosition]);return n,e==nil
}

var _ Adapter = (*SNMPAdapter)(nil)
