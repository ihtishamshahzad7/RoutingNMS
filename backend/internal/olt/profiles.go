package olt

import "strings"

// VendorProfile identifies the OID mapping used by a device. OIDs are kept in
// configuration rather than embedded in the polling engine because vendor and
// firmware MIB layouts vary significantly.
type VendorProfile struct {
	Name string
	Vendor string
	Models []string
	Mapping OIDMapping
}

type ProfileRegistry struct { profiles []VendorProfile }

func NewProfileRegistry(profiles ...VendorProfile) *ProfileRegistry {
	return &ProfileRegistry{profiles: profiles}
}

func (r *ProfileRegistry) Match(vendor, model string) (VendorProfile, bool) {
	vendor, model = strings.ToLower(strings.TrimSpace(vendor)), strings.ToLower(strings.TrimSpace(model))
	for _, p := range r.profiles {
		if strings.ToLower(p.Vendor) != vendor { continue }
		if len(p.Models) == 0 { return p, true }
		for _, m := range p.Models { if strings.EqualFold(strings.TrimSpace(m), model) { return p, true } }
	}
	return VendorProfile{}, false
}

// Profiles are intentionally templates. Populate exact OIDs from the target
// OLT's supported MIB/firmware before enabling a profile in production.
var DefaultProfiles = []VendorProfile{
	{Name:"ZTE SNMP", Vendor:"zte"},
	{Name:"Huawei SNMP", Vendor:"huawei"},
	{Name:"FiberHome SNMP", Vendor:"fiberhome"},
}
