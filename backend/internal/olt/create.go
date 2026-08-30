package olt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// CreateInput is what an operator submits from the "Add OLT" form.
type CreateInput struct {
	Name                string `json:"name"`
	Address             string `json:"address"`
	Vendor              string `json:"vendor"`
	Model               string `json:"model"`
	Serial              string `json:"serial"`
	SNMPVersion         string `json:"snmpVersion"`
	SNMPCommunity       string `json:"snmpCommunity"`
	PollIntervalSeconds int    `json:"pollIntervalSeconds"`
}

func newOLTID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "olt-" + hex.EncodeToString(b)
}

// Create validates input and inserts a new OLT row. It does not itself start
// polling — the caller (the HTTP handler) is responsible for wiring the
// returned OLT into a RuntimeManager once the row exists, since only the
// caller has access to the running RuntimeManager instance.
func (s ConfigService) Create(ctx context.Context, in CreateInput) (OLT, error) {
	if s.DB == nil {
		return OLT{}, fmt.Errorf("database is not initialized")
	}
	name := strings.TrimSpace(in.Name)
	address := strings.TrimSpace(in.Address)
	vendor := strings.TrimSpace(in.Vendor)
	if name == "" {
		return OLT{}, fmt.Errorf("name is required")
	}
	if address == "" {
		return OLT{}, fmt.Errorf("address is required")
	}
	if vendor == "" {
		return OLT{}, fmt.Errorf("vendor is required")
	}
	version := strings.TrimSpace(in.SNMPVersion)
	if version == "" {
		version = "2c"
	}
	if version != "2c" && version != "3" {
		return OLT{}, fmt.Errorf("SNMP version must be 2c or 3")
	}
	pollSeconds := in.PollIntervalSeconds
	if pollSeconds == 0 {
		pollSeconds = 60
	}
	if pollSeconds < 30 {
		return OLT{}, fmt.Errorf("poll interval must be at least 30 seconds")
	}

	id := newOLTID()
	_, err := s.DB.Exec(ctx, `
		INSERT INTO olts (id, name, address, vendor, model, serial, enabled, snmp_version, snmp_community, poll_interval_seconds)
		VALUES ($1,$2,$3,$4,$5,$6,true,$7,$8,$9)`,
		id, name, address, vendor, in.Model, in.Serial, version, in.SNMPCommunity, pollSeconds)
	if err != nil {
		return OLT{}, fmt.Errorf("save OLT: %w", err)
	}
	return OLT{ID: id, Name: name, Address: address, Vendor: vendor, Model: in.Model, Serial: in.Serial, Enabled: true}, nil
}

// List returns every configured OLT, enabled or not, for inventory views.
// (LoadEnabled, used by the polling runtime, only returns enabled ones.)
func (s ConfigService) List(ctx context.Context) ([]OLT, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	rows, err := s.DB.Query(ctx, `SELECT id,name,address,vendor,model,serial,enabled FROM olts ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OLT{}
	for rows.Next() {
		var o OLT
		if err := rows.Scan(&o.ID, &o.Name, &o.Address, &o.Vendor, &o.Model, &o.Serial, &o.Enabled); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// LoadOne resolves a single OLT by ID into a pollable ConfiguredOLT, the same
// way LoadEnabled resolves every enabled OLT at startup. Used to start
// polling immediately after an operator adds a new OLT, without restarting
// the API process.
func (s ConfigService) LoadOne(ctx context.Context, id string) (ConfiguredOLT, error) {
	if s.DB == nil {
		return ConfiguredOLT{}, fmt.Errorf("database is not initialized")
	}
	if s.Profiles == nil {
		return ConfiguredOLT{}, fmt.Errorf("OLT profile registry is not initialized")
	}
	row := s.DB.QueryRow(ctx, `SELECT id,name,address,vendor,model,serial,enabled,snmp_version,snmp_community,snmp_username,snmp_auth_protocol,snmp_auth_password,snmp_priv_protocol,snmp_priv_password,poll_interval_seconds,profile_name FROM olts WHERE id=$1`, id)
	return scanConfiguredOLT(row, s.Profiles)
}
