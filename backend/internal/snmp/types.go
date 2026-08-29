package snmp

import "time"

type Version string

const (
	V2c Version = "2c"
	V3  Version = "3"
)

type Credentials struct {
	Version   Version
	Community string
	Username  string
	AuthProto string
	AuthPass  string
	PrivProto string
	PrivPass  string
}

type Target struct {
	ID          string
	Address     string
	Port        uint16
	Credentials Credentials
	Timeout     time.Duration
	Retries     int
}

type Value struct {
	OID       string
	Index     string
	Name      string
	Value     any
	Timestamp time.Time
}
