package snmp

import "time"

// Value is the canonical result returned by SNMP GET/WALK operations.
type Value struct {
	OID       string
	Value     any
	Timestamp time.Time
}
