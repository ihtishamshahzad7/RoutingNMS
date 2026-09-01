package sites

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Site is a physical branch/site location (migration 0018). Tenant scoping
// uses a plain-text id (same convention as devices.organization_id).
type Site struct {
	ID        int64      `json:"id"`
	TenantID  string     `json:"tenantId"`
	Name      string     `json:"name"`
	Code      string     `json:"code"`
	Address   string     `json:"address"`
	City      string     `json:"city"`
	Country   string     `json:"country"`
	Latitude  *float64   `json:"latitude,omitempty"`
	Longitude *float64   `json:"longitude,omitempty"`
	Timezone  string     `json:"timezone"`
	IsActive  bool       `json:"isActive"`
	Notes     string     `json:"notes"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) List(ctx context.Context, tenantID string) ([]Site, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("sites repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,tenant_id,name,code,address,city,country,latitude,longitude,timezone,is_active,notes,created_at,updated_at
		FROM sites WHERE ($1 = '' OR tenant_id = $1) ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Site{}
	for rows.Next() {
		var s Site
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Code, &s.Address, &s.City,
			&s.Country, &s.Latitude, &s.Longitude, &s.Timezone, &s.IsActive, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r Repository) Get(ctx context.Context, id int64) (Site, error) {
	if r.DB == nil {
		return Site{}, fmt.Errorf("sites repository is not initialized")
	}
	var s Site
	err := r.DB.QueryRow(ctx, `SELECT id,tenant_id,name,code,address,city,country,latitude,longitude,timezone,is_active,notes,created_at,updated_at
		FROM sites WHERE id=$1`, id).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Code, &s.Address, &s.City,
			&s.Country, &s.Latitude, &s.Longitude, &s.Timezone, &s.IsActive, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// SiteInput is the subset of site fields an operator supplies on create/update.
type SiteInput struct {
	TenantID  string   `json:"tenantId"`
	Name      string   `json:"name"`
	Code      string   `json:"code"`
	Address   string   `json:"address"`
	City      string   `json:"city"`
	Country   string   `json:"country"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Timezone  string   `json:"timezone"`
	IsActive  *bool    `json:"isActive,omitempty"`
	Notes     string   `json:"notes"`
}

func (r Repository) Create(ctx context.Context, in SiteInput) (Site, error) {
	if r.DB == nil {
		return Site{}, fmt.Errorf("sites repository is not initialized")
	}
	if strings.TrimSpace(in.Name) == "" {
		return Site{}, fmt.Errorf("site name is required")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	var s Site
	err := r.DB.QueryRow(ctx, `INSERT INTO sites (tenant_id,name,code,address,city,country,latitude,longitude,timezone,is_active,notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id,tenant_id,name,code,address,city,country,latitude,longitude,timezone,is_active,notes,created_at,updated_at`,
		in.TenantID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Code), in.Address, in.City, in.Country,
		in.Latitude, in.Longitude, tzOrDefault(in.Timezone), active, in.Notes).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Code, &s.Address, &s.City,
			&s.Country, &s.Latitude, &s.Longitude, &s.Timezone, &s.IsActive, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (r Repository) Update(ctx context.Context, id int64, in SiteInput) (Site, error) {
	if r.DB == nil {
		return Site{}, fmt.Errorf("sites repository is not initialized")
	}
	if strings.TrimSpace(in.Name) == "" {
		return Site{}, fmt.Errorf("site name is required")
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	var s Site
	err := r.DB.QueryRow(ctx, `UPDATE sites SET tenant_id=$2,name=$3,code=$4,address=$5,city=$6,country=$7,
		latitude=$8,longitude=$9,timezone=$10,is_active=$11,notes=$12,updated_at=NOW()
		WHERE id=$1
		RETURNING id,tenant_id,name,code,address,city,country,latitude,longitude,timezone,is_active,notes,created_at,updated_at`,
		id, in.TenantID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Code), in.Address, in.City, in.Country,
		in.Latitude, in.Longitude, tzOrDefault(in.Timezone), active, in.Notes).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Code, &s.Address, &s.City,
			&s.Country, &s.Latitude, &s.Longitude, &s.Timezone, &s.IsActive, &s.Notes,
			&s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("sites repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM sites WHERE id=$1`, id)
	return err
}

func tzOrDefault(tz string) string {
	if strings.TrimSpace(tz) == "" {
		return "UTC"
	}
	return tz
}
