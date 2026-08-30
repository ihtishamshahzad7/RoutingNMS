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

func (a *SNMPAdapter) Discover(ctx context.Context, olt OLT) ([]PONPort, error) {
	if a.Mapping.PONName == "" { return nil, fmt.Errorf("PON OID mapping is not configured") }
	values, err := a.Collector.Walk(ctx, a.targetFor(olt), a.Mapping.PONName)
	if err != nil { return nil, fmt.Errorf("walk PON names: %w", err) }
	now := time.Now().UTC(); pons := make([]PONPort, 0, len(values))
	for i, v := range values { idx:=onuIndex(v.OID,a.Mapping.PONName); port:=i+1; if n,ok:=parseIndexNumber(idx);ok{port=n}; pons=append(pons,PONPort{ID:v.OID,OLTID:olt.ID,Name:fmt.Sprint(v.Value),Index:port,Port:port,Type:"pon",Status:Unknown,LastSeen:&now}) }
	return pons,nil
}

func (a *SNMPAdapter) DiscoverONUs(ctx context.Context, olt OLT, port PONPort) ([]ONU,error) {
	if a.Mapping.ONUSerial=="" { return nil,fmt.Errorf("ONU serial OID mapping is not configured") }
	if !a.Mapping.ONUIndex.Valid() { return nil,fmt.Errorf("ONU index mapping is not configured") }
	values,err:=a.Collector.Walk(ctx,a.targetFor(olt),a.Mapping.ONUSerial);if err!=nil{return nil,fmt.Errorf("walk ONU serials on %s: %w",port.Name,err)}
	ponIndex:=onuIndex(port.ID,a.Mapping.PONName);if ponIndex==""{return nil,fmt.Errorf("cannot derive PON table index from %q",port.ID)}
	onus:=make([]ONU,0)
	for _,v:=range values{idx:=onuIndex(v.OID,a.Mapping.ONUSerial);parent,_,e:=a.Mapping.ONUIndex.Extract(idx);if e!=nil{continue};if !sameIndex(parent,ponIndex){continue};onus=append(onus,ONU{ID:v.OID,OLTID:olt.ID,PONPortID:port.ID,Serial:strings.TrimSpace(fmt.Sprint(v.Value)),Status:Unknown})}
	return onus,nil
}

func (a *SNMPAdapter) PollONU(ctx context.Context, olt OLT, onu ONU)(ONU,error){select{case <-ctx.Done():return onu,ctx.Err();default:};index:=onuIndex(onu.ID,a.Mapping.ONUSerial);if index==""{return onu,fmt.Errorf("cannot derive ONU table index from %q",onu.ID)};parts:=strings.Split(index,".");type metric struct{template OIDTemplate;set func(snmp.Value)};metrics:=[]metric{{a.Mapping.ONUStatusOID,func(v snmp.Value){onu.Status=parseStatus(v.Value)}},{a.Mapping.ONULOSOID,func(v snmp.Value){onu.LOS=parseBool(v.Value)}},{a.Mapping.ONURXPowerOID,func(v snmp.Value){if n,ok:=parseFloat(v.Value);ok{onu.RxPowerDBm=&n}}},{a.Mapping.ONUTXPowerOID,func(v snmp.Value){if n,ok:=parseFloat(v.Value);ok{onu.TxPowerDBm=&n}}}};oids:=make([]string,0,len(metrics));active:=make([]metric,0,len(metrics));for _,m:=range metrics{if !m.template.Valid(){continue};oid,e:=m.template.Indexed(parts...);if e!=nil{return onu,e};oids=append(oids,oid);active=append(active,m)};if len(oids)==0{return onu,fmt.Errorf("no ONU metric OID templates configured")};values,e:=a.Collector.Get(ctx,a.targetFor(olt),oids);if e!=nil{return onu,e};for i,v:=range values{if i<len(active){active[i].set(v)}};now:=time.Now().UTC();onu.LastSeen=&now;return onu,nil}
func(a *SNMPAdapter)targetFor(olt OLT)snmp.Target{t:=a.Target;if strings.TrimSpace(olt.Address)!=""{t.Address=olt.Address};return t}
func onuIndex(id,base string)string{id=strings.Trim(id,".");base=strings.TrimRight(strings.TrimSpace(base),".");if base!=""{p:=base+".";if strings.HasPrefix(id,p){return strings.TrimPrefix(id,p)}};return ""}
func parseIndexNumber(index string)(int,bool){p:=strings.Split(strings.Trim(index,"."),".");if len(p)==0{return 0,false};n,e:=strconv.Atoi(p[len(p)-1]);return n,e==nil}
func sameIndex(a,b string)bool{a=strings.Trim(a,".");b=strings.Trim(b,".");return a!=""&&b!=""&&(a==b||strings.TrimPrefix(a,"0")==strings.TrimPrefix(b,"0"))}
func parseFloat(v any)(float64,bool){switch x:=v.(type){case float64:return x,true;case float32:return float64(x),true;case int:return float64(x),true;case int64:return float64(x),true;case uint64:return float64(x),true;case string:n,e:=strconv.ParseFloat(strings.TrimSpace(x),64);return n,e==nil};return 0,false}
func parseBool(v any)bool{switch x:=v.(type){case bool:return x;case int:return x!=0;case int64:return x!=0;case uint64:return x!=0;case string:s:=strings.ToLower(strings.TrimSpace(x));return s=="1"||s=="true"||s=="yes"||s=="on"||s=="los"};return false}
func parseStatus(v any)Status{if n,ok:=parseFloat(v);ok{if n==1{return Online};if n==0||n==2||n==3{return Offline}};s:=strings.ToLower(strings.TrimSpace(fmt.Sprint(v)));if strings.Contains(s,"online")||strings.Contains(s,"up"){return Online};if strings.Contains(s,"offline")||strings.Contains(s,"down"){return Offline};return Unknown}
var _ Adapter=(*SNMPAdapter)(nil)
