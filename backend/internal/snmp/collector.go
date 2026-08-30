package snmp

import (
	"context"
	"fmt"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

type Collector struct{}

func (Collector) Connect(ctx context.Context, target Target) (*gosnmp.GoSNMP, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	target = target.Normalize()
	if err := target.Validate(); err != nil {
		return nil, err
	}

	version := gosnmp.Version2c
	if target.Credentials.Version == V3 {
		version = gosnmp.Version3
	}

	client := &gosnmp.GoSNMP{
		Target:  target.Address,
		Port:    target.Port,
		Timeout: target.Timeout,
		Retries: max(0, target.Retries),
		Version: version,
		MaxOids: gosnmp.MaxOids,
	}
	if target.Credentials.Version == V2c {
		client.Community = target.Credentials.Community
	}
	if target.Credentials.Version == V3 {
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 target.Credentials.Username,
			AuthenticationProtocol:   authProtocol(target.Credentials.AuthProto),
			PrivacyProtocol:          privProtocol(target.Credentials.PrivProto),
			AuthenticationPassphrase: target.Credentials.AuthPass,
			PrivacyPassphrase:        target.Credentials.PrivPass,
		}
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("snmp connect %s: %w", target.Address, err)
	}
	return client, nil
}

func (Collector) Get(ctx context.Context, target Target, oids []string) ([]Value, error) {
	if len(oids) == 0 {
		return nil, fmt.Errorf("SNMP OID list is empty")
	}
	client, err := (Collector{}).Connect(ctx, target)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	packet, err := client.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("snmp get %s: %w", target.Address, err)
	}
	return packetValues(packet), nil
}

func (Collector) Walk(ctx context.Context, target Target, oid string) ([]Value, error) {
	if oid == "" {
		return nil, fmt.Errorf("SNMP walk OID is required")
	}
	client, err := (Collector{}).Connect(ctx, target)
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()
	values := make([]Value, 0)
	err = client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		values = append(values, Value{OID: pdu.Name, Value: pdu.Value, Timestamp: time.Now().UTC()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snmp walk %s %s: %w", target.Address, oid, err)
	}
	return values, nil
}

func packetValues(packet *gosnmp.SnmpPacket) []Value {
	now := time.Now().UTC()
	values := make([]Value, 0, len(packet.Variables))
	for _, variable := range packet.Variables {
		values = append(values, Value{OID: variable.Name, Value: variable.Value, Timestamp: now})
	}
	return values
}

func authProtocol(value string) gosnmp.SnmpV3AuthProtocol {
	switch value {
	case "MD5":
		return gosnmp.MD5
	case "SHA":
		return gosnmp.SHA
	case "SHA224":
		return gosnmp.SHA224
	case "SHA256":
		return gosnmp.SHA256
	case "SHA384":
		return gosnmp.SHA384
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func privProtocol(value string) gosnmp.SnmpV3PrivProtocol {
	switch value {
	case "DES":
		return gosnmp.DES
	case "AES":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES256":
		return gosnmp.AES256
	default:
		return gosnmp.NoPriv
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
