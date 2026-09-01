package customers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connection is a subscriber customer connection (migration 0018), linked to
// an optional access point and an optional device (the CPE gateway).
type Connection struct {
	ID             int64      `json:"id"`
	CustomerName   string     `json:"customerName"`
	CustomerCode   string     `json:"customerCode"`
	AccessPointID  *int64     `json:"accessPointId,omitempty"`
	DeviceID       *int64     `json:"deviceId,omitempty"`
	PlanName       string     `json:"planName"`
	IPAddress      string     `json:"ipAddress"`
	MACAddress     string     `json:"macAddress"`
	BandwidthDLMbps float64   `json:"bandwidthDlMbps"`
	BandwidthULMbps float64   `json:"bandwidthUlMbps"`
	IsActive       bool       `json:"isActive"`
	ContractStart  *time.Time `json:"contractStart,omitempty"`
	ContractEnd    *time.Time `json:"contractEnd,omitempty"`
	Notes          string     `json:"notes"`
	Latitude       *float64   `json:"latitude,omitempty"`
	Longitude      *float64   `json:"longitude,omitempty"`
	TenantID       string     `json:"tenantId"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type Repository struct{ DB *pgxpool.Pool }

const selectCols = `id,customer_name,customer_code,access_point_id,device_id,plan_name,ip_address,mac_address,bandwidth_dl_mbps,bandwidth_ul_mbps,is_active,contract_start,contract_end,notes,latitude,longitude,tenant_id,created_at,updated_at`

func (r Repository) List(ctx context.Context, tenantID string) ([]Connection, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("customers repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT `+selectCols+`
		FROM customer_connections
		WHERE ($1 = '' OR tenant_id = $1) ORDER BY customer_name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Connection{}
	for rows.Next() {
		var c Connection
		if err := scanConn(rows, &c); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r Repository) Get(ctx context.Context, id int64) (Connection, error) {
	if r.DB == nil {
		return Connection{}, fmt.Errorf("customers repository is not initialized")
	}
	row := r.DB.QueryRow(ctx, `SELECT `+selectCols+` FROM customer_connections WHERE id=$1`, id)
	var c Connection
	if err := scanConn(row, &c); err != nil {
		return Connection{}, err
	}
	return c, nil
}

// Input is the subset of fields an operator supplies on create/update.
type Input struct {
	CustomerName    string     `json:"customerName"`
	CustomerCode    string     `json:"customerCode"`
	AccessPointID   *int64     `json:"accessPointId,omitempty"`
	DeviceID        *int64     `json:"deviceId,omitempty"`
	PlanName        string     `json:"planName"`
	IPAddress       string     `json:"ipAddress"`
	MACAddress      string     `json:"macAddress"`
	BandwidthDLMbps float64    `json:"bandwidthDlMbps"`
	BandwidthULMbps float64    `json:"bandwidthUlMbps"`
	IsActive        *bool      `json:"isActive,omitempty"`
	ContractStart   *time.Time `json:"contractStart,omitempty"`
	ContractEnd     *time.Time `json:"contractEnd,omitempty"`
	Notes           string     `json:"notes"`
	Latitude        *float64   `json:"latitude,omitempty"`
	Longitude       *float64   `json:"longitude,omitempty"`
	TenantID        string     `json:"tenantId"`
}

func (r Repository) Create(ctx context.Context, in Input) (Connection, error) {
	if r.DB == nil {
		return Connection{}, fmt.Errorf("customers repository is not initialized")
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return Connection{}, fmt.Errorf("customer name is required")
	}
	if strings.TrimSpace(in.CustomerCode) == "" {
		return Connection{}, fmt.Errorf("customer code is required")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	row := r.DB.QueryRow(ctx, `INSERT INTO customer_connections
		(customer_name,customer_code,access_point_id,device_id,plan_name,ip_address,mac_address,bandwidth_dl_mbps,bandwidth_ul_mbps,is_active,contract_start,contract_end,notes,latitude,longitude,tenant_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING `+selectCols,
		strings.TrimSpace(in.CustomerName), strings.TrimSpace(in.CustomerCode), in.AccessPointID, in.DeviceID,
		in.PlanName, strings.TrimSpace(in.IPAddress), strings.TrimSpace(in.MACAddress),
		in.BandwidthDLMbps, in.BandwidthULMbps, active, in.ContractStart, in.ContractEnd,
		in.Notes, in.Latitude, in.Longitude, in.TenantID)
	var c Connection
	if err := scanConn(row, &c); err != nil {
		return Connection{}, err
	}
	return c, nil
}

func (r Repository) Update(ctx context.Context, id int64, in Input) (Connection, error) {
	if r.DB == nil {
		return Connection{}, fmt.Errorf("customers repository is not initialized")
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return Connection{}, fmt.Errorf("customer name is required")
	}
	if strings.TrimSpace(in.CustomerCode) == "" {
		return Connection{}, fmt.Errorf("customer code is required")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	row := r.DB.QueryRow(ctx, `UPDATE customer_connections SET
		customer_name=$2,customer_code=$3,access_point_id=$4,device_id=$5,plan_name=$6,ip_address=$7,
		mac_address=$8,bandwidth_dl_mbps=$9,bandwidth_ul_mbps=$10,is_active=$11,contract_start=$12,
		contract_end=$13,notes=$14,latitude=$15,longitude=$16,tenant_id=$17,updated_at=NOW()
		WHERE id=$1
		RETURNING `+selectCols,
		id, strings.TrimSpace(in.CustomerName), strings.TrimSpace(in.CustomerCode), in.AccessPointID, in.DeviceID,
		in.PlanName, strings.TrimSpace(in.IPAddress), strings.TrimSpace(in.MACAddress),
		in.BandwidthDLMbps, in.BandwidthULMbps, active, in.ContractStart, in.ContractEnd,
		in.Notes, in.Latitude, in.Longitude, in.TenantID)
	var c Connection
	if err := scanConn(row, &c); err != nil {
		return Connection{}, err
	}
	return c, nil
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("customers repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM customer_connections WHERE id=$1`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanConn(s scanner, c *Connection) error {
	return s.Scan(&c.ID, &c.CustomerName, &c.CustomerCode, &c.AccessPointID, &c.DeviceID,
		&c.PlanName, &c.IPAddress, &c.MACAddress, &c.BandwidthDLMbps, &c.BandwidthULMbps,
		&c.IsActive, &c.ContractStart, &c.ContractEnd, &c.Notes, &c.Latitude, &c.Longitude,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt)
}
