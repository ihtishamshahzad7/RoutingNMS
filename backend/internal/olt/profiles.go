package olt

import (
	"fmt"
	"strings"
)

// OIDTemplate describes a vendor MIB column and how its table index is built.
type OIDTemplate struct {
	Base string
	IndexOrder []string
}

func (t OIDTemplate) Valid() bool { return strings.TrimSpace(t.Base) != "" }

// Indexed builds an OID only when a profile explicitly defines the index order.
// This prevents the polling engine from guessing vendor-specific ONU indexes.
func (t OIDTemplate) Indexed(index ...string) (string, error) {
	if !t.Valid() { return "", fmt.Errorf("OID template base is empty") }
	if len(t.IndexOrder) != len(index) { return "", fmt.Errorf("OID template expects %d indexes, got %d", len(t.IndexOrder), len(index)) }
	base := strings.TrimRight(strings.TrimSpace(t.Base), ".")
	for _, v := range index { if strings.TrimSpace(v)=="" { return "",fmt.Errorf("OID index cannot be empty") }; base += "."+strings.TrimSpace(v) }
	return base,nil
}

type VendorProfile struct {
	Name string
	Vendor string
	Models []string
	PON OIDTemplate
	ONU OIDTemplate
	Mapping OIDMapping
}

type ProfileRegistry struct { profiles []VendorProfile }

func NewProfileRegistry(profiles ...VendorProfile) *ProfileRegistry { return &ProfileRegistry{profiles: profiles} }

func (r *ProfileRegistry) Match(vendor, model string) (VendorProfile, bool) {
	vendor, model = strings.ToLower(strings.TrimSpace(vendor)), strings.ToLower(strings.TrimSpace(model))
	for _, p := range r.profiles {
		if strings.ToLower(p.Vendor) != vendor { continue }
		if len(p.Models)==0 { return p,true }
		for _, m := range p.Models { if strings.EqualFold(strings.TrimSpace(m),model) { return p,true } }
	}
	return VendorProfile{},false
}

var DefaultProfiles = []VendorProfile{
	{Name:"ZTE SNMP",Vendor:"zte"},
	{Name:"Huawei SNMP",Vendor:"huawei"},
	{Name:"FiberHome SNMP",Vendor:"fiberhome"},
}

func DefaultProfileRegistry() *ProfileRegistry { return NewProfileRegistry(DefaultProfiles...) }
func (r *ProfileRegistry) Resolve(vendor,model string)(OIDMapping,bool){p,ok:=r.Match(vendor,model);if !ok{return OIDMapping{},false};return p.Mapping,true}
