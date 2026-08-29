package olt

import "strings"

// OIDMapping contains the normalized OIDs used by an OLT vendor profile.
type OIDMapping struct {
	PONName   string
	ONUSerial string
	ONUIndex  ONUIndexSpec
}

// Valid reports whether the minimum discovery mapping is usable.
func (m OIDMapping) Valid() bool {
	return strings.TrimSpace(m.PONName) != "" &&
		strings.TrimSpace(m.ONUSerial) != "" &&
		m.ONUIndex.Valid()
}
