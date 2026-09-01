package accesspoints

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AccessPoint is a wireless access point (sector/ptp/ptmp/olt/cmts) that can
// be linked to an optional SNMP device record and an optional site (migration
// 0018). Footprint is kept free-form JSON.
type AccessPoint struct {
	ID               int64           `json:"id"`
	Name             string          `json:"name"`
	SiteID           *int64          `json:"siteId,omitempty"`
	DeviceID         *int64          `json:"deviceId,omitempty"`
	APType           string          `json:"apType"`
	FrequencyBand    string          `json:"frequencyBand"`
	Channel          string          `json:"channel"`
	TxPowerDBm       *float64        `json:"txPowerDbm,omitempty"`
	MaxClients       *int            `json:"maxClients,omitempty"`
	ParentApID       *int64          `json:"parentApId,omitempty"`
	IPAddress        string          `json:"ipAddress"`
	MACAddress       string          `json:"macAddress"`
	Latitude         *float64        `json:"latitude,omitempty"`
	Longitude        *float64        `json:"longitude,omitempty"`
	Footprint        json.RawMessage `json:"footprint,omitempty"`
	MonthlyBWLimitGB *int            `json:"monthlyBwLimitGb,omitempty"`
	TenantID         string          `json:"tenantId"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type Repository struct{ DB *pgxpool.Pool }

const selectCols = `id,name,site_id,device_id,ap_type,frequency_band,channel,tx_power_dbm,max_clients,parent_ap_id,ip_address,mac_address,latitude,longitude,footprint,monthly_bw_limit_gb,tenant_id,created_at,updated_at`

func (r Repository) List(ctx context.Context, tenantID string, siteID string) ([]AccessPoint, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("access points repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT `+selectCols+`
		FROM access_points
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR site_id = NULLIF(CAST($2 AS BIGINT),0))
		ORDER BY name`, tenantID, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []AccessPoint{}
	for rows.Next() {
		var a AccessPoint
		if err := scanAP(rows, &a); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r Repository) Get(ctx context.Context, id int64) (AccessPoint, error) {
	if r.DB == nil {
		return AccessPoint{}, fmt.Errorf("access points repository is not initialized")
	}
	row := r.DB.QueryRow(ctx, `SELECT `+selectCols+` FROM access_points WHERE id=$1`, id)
	var a AccessPoint
	if err := scanAP(row, &a); err != nil {
		return AccessPoint{}, err
	}
	return a, nil
}

// Input is the subset of fields an operator supplies on create/update.
type Input struct {
	Name             string          `json:"name"`
	SiteID           *int64          `json:"siteId,omitempty"`
	DeviceID         *int64          `json:"deviceId,omitempty"`
	APType           string          `json:"apType"`
	FrequencyBand    string          `json:"frequencyBand"`
	Channel          string          `json:"channel"`
	TxPowerDBm       *float64        `json:"txPowerDbm,omitempty"`
	MaxClients       *int            `json:"maxClients,omitempty"`
	ParentApID       *int64          `json:"parentApId,omitempty"`
	IPAddress        string          `json:"ipAddress"`
	MACAddress       string          `json:"macAddress"`
	Latitude         *float64        `json:"latitude,omitempty"`
	Longitude        *float64        `json:"longitude,omitempty"`
	Footprint        json.RawMessage `json:"footprint,omitempty"`
	MonthlyBWLimitGB *int            `json:"monthlyBwLimitGb,omitempty"`
	TenantID         string          `json:"tenantId"`
}

func (r Repository) Create(ctx context.Context, in Input) (AccessPoint, error) {
	if r.DB == nil {
		return AccessPoint{}, fmt.Errorf("access points repository is not initialized")
	}
	if strings.TrimSpace(in.Name) == "" {
		return AccessPoint{}, fmt.Errorf("access point name is required")
	}
	apType := strings.TrimSpace(in.APType)
	if apType == "" {
		apType = "sector"
	}
	var footprint any
	if len(in.Footprint) > 0 {
		if err := json.Unmarshal(in.Footprint, &footprint); err != nil {
			return AccessPoint{}, fmt.Errorf("footprint is not valid JSON: %w", err)
		}
	}
	row := r.DB.QueryRow(ctx, `INSERT INTO access_points
		(name,site_id,device_id,ap_type,frequency_band,channel,tx_power_dbm,max_clients,parent_ap_id,ip_address,mac_address,latitude,longitude,footprint,monthly_bw_limit_gb,tenant_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING `+selectCols,
		strings.TrimSpace(in.Name), in.SiteID, in.DeviceID, apType, in.FrequencyBand, in.Channel,
		in.TxPowerDBm, in.MaxClients, in.ParentApID, strings.TrimSpace(in.IPAddress), strings.TrimSpace(in.MACAddress),
		in.Latitude, in.Longitude, footprint, in.MonthlyBWLimitGB, in.TenantID)
	var a AccessPoint
	if err := scanAP(row, &a); err != nil {
		return AccessPoint{}, err
	}
	return a, nil
}

func (r Repository) Update(ctx context.Context, id int64, in Input) (AccessPoint, error) {
	if r.DB == nil {
		return AccessPoint{}, fmt.Errorf("access points repository is not initialized")
	}
	if strings.TrimSpace(in.Name) == "" {
		return AccessPoint{}, fmt.Errorf("access point name is required")
	}
	apType := strings.TrimSpace(in.APType)
	if apType == "" {
		apType = "sector"
	}
	var footprint any
	if len(in.Footprint) > 0 {
		if err := json.Unmarshal(in.Footprint, &footprint); err != nil {
			return AccessPoint{}, fmt.Errorf("footprint is not valid JSON: %w", err)
		}
	}
	row := r.DB.QueryRow(ctx, `UPDATE access_points SET
		name=$2,site_id=$3,device_id=$4,ap_type=$5,frequency_band=$6,channel=$7,tx_power_dbm=$8,
		max_clients=$9,parent_ap_id=$10,ip_address=$11,mac_address=$12,latitude=$13,longitude=$14,
		footprint=$15,monthly_bw_limit_gb=$16,tenant_id=$17,updated_at=NOW()
		WHERE id=$1
		RETURNING `+selectCols,
		id, strings.TrimSpace(in.Name), in.SiteID, in.DeviceID, apType, in.FrequencyBand, in.Channel,
		in.TxPowerDBm, in.MaxClients, in.ParentApID, strings.TrimSpace(in.IPAddress), strings.TrimSpace(in.MACAddress),
		in.Latitude, in.Longitude, footprint, in.MonthlyBWLimitGB, in.TenantID)
	var a AccessPoint
	if err := scanAP(row, &a); err != nil {
		return AccessPoint{}, err
	}
	return a, nil
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("access points repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM access_points WHERE id=$1`, id)
	return err
}

// scanner abstracts pgx.Rows and pgx.Row so scanAP works for both.
type scanner interface {
	Scan(dest ...any) error
}

func scanAP(s scanner, a *AccessPoint) error {
	var footprint []byte
	err := s.Scan(&a.ID, &a.Name, &a.SiteID, &a.DeviceID, &a.APType, &a.FrequencyBand,
		&a.Channel, &a.TxPowerDBm, &a.MaxClients, &a.ParentApID, &a.IPAddress, &a.MACAddress,
		&a.Latitude, &a.Longitude, &footprint, &a.MonthlyBWLimitGB, &a.TenantID, &a.CreatedAt, &a.UpdatedAt)
	if err == nil && len(footprint) > 0 {
		a.Footprint = footprint
	}
	return err
}
