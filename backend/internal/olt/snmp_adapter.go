package olt

import (
    "context"
    "fmt"
    "strings"

    "github.com/gosnmp/gosnmp"
    "github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

type SNMPAdapter struct { Target snmp.Target; Mapping OIDMapping; Client *gosnmp.GoSNMP }

type OIDMapping struct { PONName, PONStatus, ONUSerial, ONUStatus, ONULOS, ONURXPower, ONUTXPower string }

func (a *SNMPAdapter) Discover(ctx context.Context, olt OLT) ([]PONPort, error) {
    if a.Mapping.PONName == "" { return nil, fmt.Errorf("PON OID mapping is not configured") }
    c, err := a.connect(olt); if err != nil { return nil, err }; defer c.Conn.Close()
    var values []gosnmp.SnmpPDU
    if err := c.Walk(a.Mapping.PONName, func(pdu gosnmp.SnmpPDU) error { values=append(values,pdu); select { case <-ctx.Done(): return ctx.Err(); default: return nil } }); err != nil { return nil, fmt.Errorf("walk PON names: %w",err) }
    pons:=make([]PONPort,0,len(values)); for i,v:=range values { pons=append(pons,PONPort{ID:v.Name,OLTID:olt.ID,Name:fmt.Sprint(v.Value),Index:i+1,Type:"pon",Status:Unknown}) }
    return pons,nil
}

func (a *SNMPAdapter) DiscoverONUs(ctx context.Context, olt OLT, port PONPort) ([]ONU,error) {
    if a.Mapping.ONUSerial=="" { return nil,fmt.Errorf("ONU serial OID mapping is not configured") }
    c,err:=a.connect(olt); if err!=nil{return nil,err}; defer c.Conn.Close()
    var values []gosnmp.SnmpPDU
    if err:=c.Walk(a.Mapping.ONUSerial,func(pdu gosnmp.SnmpPDU) error { values=append(values,pdu); select {case <-ctx.Done(): return ctx.Err();default:return nil} });err!=nil{return nil,fmt.Errorf("walk ONU serials on %s: %w",port.Name,err)}
    onus:=make([]ONU,0,len(values));for _,v:=range values{onus=append(onus,ONU{ID:v.Name,OLTID:olt.ID,PONPortID:port.ID,Serial:strings.TrimSpace(fmt.Sprint(v.Value)),Status:Unknown})};return onus,nil
}

func (a *SNMPAdapter) PollONU(ctx context.Context, olt OLT, onu ONU)(ONU,error){ select{case <-ctx.Done():return onu,ctx.Err();default:}; return onu,nil }

func (a *SNMPAdapter) connect(olt OLT)(*gosnmp.GoSNMP,error){
    target:=a.Target; if strings.TrimSpace(olt.Address)!="" {target.Address=olt.Address}
    version:=gosnmp.Version2c;if target.Credentials.Version==snmp.V3{version=gosnmp.Version3}
    c:=&gosnmp.GoSNMP{Target:target.Address,Port:target.Port,Version:version,Timeout:target.Timeout,Retries:uint8(target.Retries),Logger:nil};if version==gosnmp.Version2c{c.Community=target.Credentials.Community};if err:=c.Connect();err!=nil{return nil,err};return c,nil
}

var _ Adapter = (*SNMPAdapter)(nil)
