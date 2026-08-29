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
	onus:=make([]ONU,0,len(values));for _,v:=range values{onus=append(onus,ONU{ID:v.OID,OLTID:olt.ID,PONPortID:port.ID,Serial:strings.TrimSpace(fmt.Sprint(v.Value)),Status:Unknown})};return onus,nil
}

func (a *SNMPAdapter) PollONU(ctx context.Context, olt OLT, onu ONU)(ONU,error){
	select{case <-ctx.Done():return onu,ctx.Err();default:}
	oids:=make([]string,0,4); kinds:=make([]string,0,4)
	for _,item:=range []struct{base,kind string}{{a.Mapping.ONUStatus,"status"},{a.Mapping.ONULOS,"los"},{a.Mapping.ONURXPower,"rx"},{a.Mapping.ONUTXPower,"tx"}} { if item.base!="" { oids=append(oids,indexOID(item.base,onu.ID)); kinds=append(kinds,item.kind) } }
	if len(oids)==0{return onu,nil}
	values,err:=a.Collector.Get(ctx,a.targetFor(olt),oids);if err!=nil{return onu,err}
	seen:=time.Now().UTC();for i,v:=range values{if i>=len(kinds){break};switch kinds[i]{case "status":if n,ok:=number(v.Value);ok{if n==1{onu.Status=Online}else{onu.Status=Offline}};case "los":if n,ok:=number(v.Value);ok{onu.LOS=n!=0};case "rx":if n,ok:=number(v.Value);ok{onu.RxPowerDBm=&n};case "tx":if n,ok:=number(v.Value);ok{onu.TxPowerDBm=&n}}};onu.LastSeen=&seen;return onu,nil
}

func (a *SNMPAdapter) targetFor(olt OLT) snmp.Target { t:=a.Target;if strings.TrimSpace(olt.Address)!=""{t.Address=olt.Address};return t }
func indexOID(base,index string)string{base=strings.TrimRight(strings.TrimSpace(base),".");index=strings.TrimSpace(index);if index==""{return base};return base+"."+index}
func number(v any)(float64,bool){switch x:=v.(type){case int:return float64(x),true;case int8:return float64(x),true;case int16:return float64(x),true;case int32:return float64(x),true;case int64:return float64(x),true;case uint:return float64(x),true;case uint8:return float64(x),true;case uint16:return float64(x),true;case uint32:return float64(x),true;case uint64:return float64(x),true;case float32:return float64(x),true;case float64:return x,true;case []byte:n,e:=strconv.ParseFloat(strings.TrimSpace(string(x)),64);return n,e==nil;case string:n,e:=strconv.ParseFloat(strings.TrimSpace(x),64);return n,e==nil;default:return 0,false}}

var _ Adapter = (*SNMPAdapter)(nil)
