package snmp

// Common SNMP OIDs. Vendor adapters should extend these instead of changing
// the polling contract. Values use IF-MIB, HOST-RESOURCES-MIB and standard
// SNMPv2-MIB where applicable.
const (
	SysDescrOID      = "1.3.6.1.2.1.1.1.0"
	SysObjectIDOID   = "1.3.6.1.2.1.1.2.0"
	SysUpTimeOID     = "1.3.6.1.2.1.1.3.0"
	SysNameOID       = "1.3.6.1.2.1.1.5.0"
	IfDescrOID       = "1.3.6.1.2.1.2.2.1.2"
	IfNameOID        = "1.3.6.1.2.1.31.1.1.1.1"
	IfOperStatusOID  = "1.3.6.1.2.1.2.2.1.8"
	IfAdminStatusOID = "1.3.6.1.2.1.2.2.1.7"
	IfHCInOctetsOID  = "1.3.6.1.2.1.31.1.1.1.6"
	IfHCOutOctetsOID = "1.3.6.1.2.1.31.1.1.1.10"
	IfInErrorsOID    = "1.3.6.1.2.1.2.2.1.14"
	IfOutErrorsOID   = "1.3.6.1.2.1.2.2.1.20"
	IfInDiscardsOID  = "1.3.6.1.2.1.2.2.1.13"
	IfOutDiscardsOID = "1.3.6.1.2.1.2.2.1.19"
)
