package olt

import (
	"fmt"
	"strings"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
)

// NewSNMPAdapter builds a fully validated vendor-profile-backed adapter.
// It deliberately refuses incomplete profiles instead of silently polling with guessed OIDs.
func NewSNMPAdapter(cfg ConfiguredOLT) (*SNMPAdapter, error) {
	target := cfg.SNMP.Normalize()
	if err := target.Validate(); err != nil {
		return nil, fmt.Errorf("OLT %s SNMP configuration: %w", cfg.OLT.ID, err)
	}
	profile := cfg.Profile
	if strings.TrimSpace(profile.Vendor) == "" {
		return nil, fmt.Errorf("OLT %s vendor profile is missing", cfg.OLT.ID)
	}
	mapping := profile.Mapping
	if mapping.PONName == "" && profile.PON.Valid() {
		mapping.PONName = strings.TrimRight(profile.PON.Base, ".")
	}
	if mapping.ONUSerial == "" && profile.ONU.Valid() {
		mapping.ONUSerial = strings.TrimRight(profile.ONU.Base, ".")
	}
	if !mapping.ONUIndex.Valid() {
		mapping.ONUIndex = profile.IndexSpec
	}
	if mapping.PONName == "" {
		return nil, fmt.Errorf("OLT %s profile %q has no PON discovery OID", cfg.OLT.ID, profile.Name)
	}
	if mapping.ONUSerial == "" {
		return nil, fmt.Errorf("OLT %s profile %q has no ONU serial discovery OID", cfg.OLT.ID, profile.Name)
	}
	if !mapping.ONUIndex.Valid() {
		return nil, fmt.Errorf("OLT %s profile %q has no ONU index specification", cfg.OLT.ID, profile.Name)
	}
	return &SNMPAdapter{Target: target, Mapping: mapping, Collector: snmp.Collector{}}, nil
}

// AdapterFor resolves the configured vendor/model and constructs its adapter.
func (s ConfigService) AdapterFor(cfg ConfiguredOLT) (Adapter, error) {
	adapter, err := NewSNMPAdapter(cfg)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}
