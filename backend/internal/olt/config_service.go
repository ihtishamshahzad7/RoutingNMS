package olt

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ihtishamshahzad7/RoutingNMS/backend/internal/snmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConfigService struct {
	DB       *pgxpool.Pool
	Profiles *ProfileRegistry
}
type ConfiguredOLT struct {
	OLT          OLT
	SNMP         snmp.Target
	Profile      VendorProfile
	PollInterval time.Duration
}

// rowScanner is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query),
// letting scanConfiguredOLT back both LoadEnabled (many rows) and LoadOne
// (a single row) without duplicating the column list or validation logic.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanConfiguredOLT(row rowScanner, profiles *ProfileRegistry) (ConfiguredOLT, error) {
	var o OLT
	var version, community, username, authProto, authPass, privProto, privPass, profileName string
	var pollSeconds int
	if err := row.Scan(&o.ID, &o.Name, &o.Address, &o.Vendor, &o.Model, &o.Serial, &o.Enabled, &version, &community, &username, &authProto, &authPass, &privProto, &privPass, &pollSeconds, &profileName); err != nil {
		if err == pgx.ErrNoRows {
			return ConfiguredOLT{}, fmt.Errorf("OLT not found")
		}
		return ConfiguredOLT{}, err
	}
	if strings.TrimSpace(o.ID) == "" || strings.TrimSpace(o.Address) == "" || strings.TrimSpace(o.Vendor) == "" {
		return ConfiguredOLT{}, fmt.Errorf("invalid OLT configuration id=%q: id, address and vendor are required", o.ID)
	}
	profile, ok := profiles.ResolveProfile(o.Vendor, o.Model)
	if !ok {
		return ConfiguredOLT{}, fmt.Errorf("no OLT profile for vendor=%q model=%q", o.Vendor, o.Model)
	}
	if strings.TrimSpace(profileName) != "" && !strings.EqualFold(profileName, profile.Name) {
		return ConfiguredOLT{}, fmt.Errorf("OLT %s requests unknown profile %q", o.ID, profileName)
	}
	if pollSeconds < 30 {
		return ConfiguredOLT{}, fmt.Errorf("OLT %s poll interval must be at least 30 seconds", o.ID)
	}
	t := snmp.Target{Address: o.Address, Port: snmp.DefaultPort, Timeout: snmp.DefaultTimeout, Retries: snmp.DefaultRetries, Credentials: snmp.Credentials{Version: snmp.Version(version), Community: community, Username: username, AuthProto: authProto, AuthPass: authPass, PrivProto: privProto, PrivPass: privPass}}.Normalize()
	if err := t.Validate(); err != nil {
		return ConfiguredOLT{}, fmt.Errorf("OLT %s: %w", o.ID, err)
	}
	return ConfiguredOLT{OLT: o, SNMP: t, Profile: profile, PollInterval: time.Duration(pollSeconds) * time.Second}, nil
}

func (s ConfigService) LoadEnabled(ctx context.Context) ([]ConfiguredOLT, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	if s.Profiles == nil {
		return nil, fmt.Errorf("OLT profile registry is not initialized")
	}
	rows, err := s.DB.Query(ctx, `SELECT id,name,address,vendor,model,serial,enabled,snmp_version,snmp_community,snmp_username,snmp_auth_protocol,snmp_auth_password,snmp_priv_protocol,snmp_priv_password,poll_interval_seconds,profile_name FROM olts WHERE enabled=true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ConfiguredOLT, 0)
	for rows.Next() {
		cfg, err := scanConfiguredOLT(rows, s.Profiles)
		if err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
