package devices

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct { DB *pgxpool.Pool }

type Record struct {
	ID string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name string `json:"name"`
	Address string `json:"address"`
	DeviceType string `json:"deviceType"`
	Vendor string `json:"vendor,omitempty"`
	Model string `json:"model,omitempty"`
	SerialNumber string `json:"serialNumber,omitempty"`
	Enabled bool `json:"enabled"`
	MonitoringIntervalSeconds int `json:"monitoringIntervalSeconds"`
}

func (r Repository) Create(ctx context.Context, in DeviceInput) (Record, error) {
	if r.DB == nil { return Record{}, fmt.Errorf("device repository is not initialized") }
	var out Record
	err := r.DB.QueryRow(ctx, `INSERT INTO devices (organization_id,name,address,device_type,vendor,enabled) VALUES ($1,$2,$3,$4,$5,true) RETURNING id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds`, in.OrganizationID, in.Name, in.Address, in.DeviceType, in.Vendor).Scan(&out.ID,&out.OrganizationID,&out.Name,&out.Address,&out.DeviceType,&out.Vendor,&out.Model,&out.SerialNumber,&out.Enabled,&out.MonitoringIntervalSeconds)
	return out, err
}

func (r Repository) List(ctx context.Context, organizationID string) ([]Record, error) {
	if r.DB == nil { return nil, fmt.Errorf("device repository is not initialized") }
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,address,device_type,COALESCE(vendor,''),COALESCE(model,''),COALESCE(serial_number,''),enabled,monitoring_interval_seconds FROM devices WHERE organization_id=$1 ORDER BY name`, organizationID)
	if err != nil { return nil, err }; defer rows.Close()
	items := []Record{}
	for rows.Next() { var d Record; if err := rows.Scan(&d.ID,&d.OrganizationID,&d.Name,&d.Address,&d.DeviceType,&d.Vendor,&d.Model,&d.SerialNumber,&d.Enabled,&d.MonitoringIntervalSeconds); err != nil { return nil, err }; items = append(items,d) }
	return items, rows.Err()
}
