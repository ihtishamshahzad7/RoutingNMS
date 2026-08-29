package olt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// SNMPAdapter is a vendor-neutral starting adapter for OLTs that expose
// standards/vendor MIBs through SNMP. OID mapping is injected per vendor/model
// instead of hard-coding ZTE/Huawei assumptions into the polling engine.
type SNMPAdapter struct {
	Target snmp.Target
	Mapping OIDMapping
	Client *gosnmp.GoSNMP
}

type OIDMapping struct {
	PONName string
	PONStatus string
	ONUSerial string
	ONUStatus string
	ONULOS string
	ONURXPower string
	ONUTXPower string
}

func (a *SNMPAdapter) DiscoverPONs() ([]PON, error) {
	if a.Mapping.PONName == "" { return nil, fmt.Errorf("PON OID mapping is not configured") }
	c, err := a.connect(); if err != nil { return nil, err }; defer c.Conn.Close()
	pdu, err := c.WalkAll(a.Mapping.PONName); if err != nil { return nil, fmt.Errorf("walk PON names: %w", err) }
	pons := make([]PON, 0, len(pdu))
	for _, v := range pdu { pons = append(pons, PON{ID:v.Name, Name:fmt.Sprint(v.Value), Status:Unknown}) }
	return pons, nil
}

func (a *SNMPAdapter) DiscoverONUs(pon PON) ([]ONU, error) {
	if a.Mapping.ONUSerial == "" { return nil, fmt.Errorf("ONU serial OID mapping is not configured") }
	c, err := a.connect(); if err != nil { return nil, err }; defer c.Conn.Close()
	items, err := c.WalkAll(a.Mapping.ONUSerial); if err != nil { return nil, fmt.Errorf("walk ONU serials: %w", err) }
	onus := make([]ONU, 0, len(items))
	for _, v := range items { onus = append(onus, ONU{ID:v.Name, SerialNumber:strings.TrimSpace(fmt.Sprint(v.Value)), Status:Unknown}) }
	return onus, nil
}

func (a *SNMPAdapter) PollONU(onu ONU) (ONU, error) { return onu, nil }

func (a *SNMPAdapter) connect() (*gosnmp.GoSNMP, error) {
	version := gosnmp.Version2c
	if a.Target.Credentials.Version == snmp.V3 { version = gosnmp.Version3 }
	c := &gosnmp.GoSNMP{Target:a.Target.Address, Port:a.Target.Port, Version:version, Timeout:a.Target.Timeout, Retries:uint8(a.Target.Retries), Logger:nil}
	if version == gosnmp.Version2c { c.Community=a.Target.Credentials.Community }
	if err := c.Connect(); err != nil { return nil, err }; return c, nil
}

var _ = context.Background
var _ = time.Second
