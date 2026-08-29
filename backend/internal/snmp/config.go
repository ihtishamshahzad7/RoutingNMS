package snmp

import "time"

// Version identifies the SNMP protocol version used by a target.
type Version string

const (
	V2c Version = "2c"
	V3  Version = "3"
)

type Credentials struct {
	Version    Version
	Community  string
	Username   string
	AuthProto  string
	AuthPass   string
	PrivProto  string
	PrivPass   string
}

type Target struct {
	Address     string
	Port        uint16
	Timeout     time.Duration
	Retries     int
	Credentials Credentials
}
