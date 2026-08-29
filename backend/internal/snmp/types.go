package snmp

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Version string
const ( V2c Version = "2c"; V3 Version = "3" )
const ( DefaultPort uint16 = 161; DefaultTimeout = 5*time.Second; DefaultRetries = 2 )

type Credentials struct { Version Version; Community string; Username string; AuthProto string; AuthPass string; PrivProto string; PrivPass string }

type Target struct { ID string; Address string; Port uint16; Credentials Credentials; Timeout time.Duration; Retries int }

func (t Target) Normalize() Target {
	t.Address=strings.TrimSpace(t.Address); t.Port=defaultPort(t.Port); if t.Timeout<=0 {t.Timeout=DefaultTimeout}; if t.Retries<0 {t.Retries=0}; t.Credentials.Version=normalizeVersion(t.Credentials.Version); return t
}
func (t Target) Validate() error {
	t=t.Normalize(); if t.Address=="" {return fmt.Errorf("SNMP target address is required")}; if net.ParseIP(t.Address)==nil && !strings.Contains(t.Address,".") {return fmt.Errorf("invalid SNMP target address %q",t.Address)}
	if t.Port==0 {return fmt.Errorf("SNMP target port is invalid")}; if t.Timeout<=0 {return fmt.Errorf("SNMP timeout must be positive")}; if t.Retries<0 {return fmt.Errorf("SNMP retries cannot be negative")}
	switch t.Credentials.Version {case V2c: if strings.TrimSpace(t.Credentials.Community)=="" {return fmt.Errorf("SNMP v2c community is required")}; case V3: if strings.TrimSpace(t.Credentials.Username)=="" {return fmt.Errorf("SNMP v3 username is required")}; default:return fmt.Errorf("unsupported SNMP version %q",t.Credentials.Version)}
	return nil
}
func defaultPort(p uint16)uint16{if p==0{return DefaultPort};return p}
func normalizeVersion(v Version)Version{v=Version(strings.ToLower(strings.TrimSpace(string(v))));if v=="v2c"||v=="2"||v=="2c"{return V2c};if v=="v3"||v=="3"{return V3};return v}

type Value struct { OID string; Index string; Name string; Value any; Timestamp time.Time }
