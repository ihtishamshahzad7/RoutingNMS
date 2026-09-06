// Package workspacetopology implements persistent storage for the Workspace
// Topology Builder (the EVE-NG/Dude-style canvas at
// frontend/app/(noc)/workspace-topology). It shipped originally with
// in-memory Zustand state only; this closes that gap using the same
// Go/Postgres stack as the rest of RoutingNMS, per migration
// 0042_workspace_topology.sql.
//
// Canvas devices/links use client-generated text ids (the frontend already
// mints ids like "dev-<timestamp>-<counter>" via nextId()) so the store's
// existing optimistic-update flow doesn't need a client/server id
// reconciliation rewrite -- Create just persists the id the frontend already
// picked. Groups are the one bigserial-id table here (mirroring
// device_groups), so GroupDTO carries the id as a string to match the
// frontend's WorkspaceGroup.id:string type exactly.
package workspacetopology

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID        int64
	TenantID  string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Device struct {
	ID             string
	GroupID        int64
	Name           string
	Kind           string
	Address        string
	SNMPCommunity  string
	PosX           float64
	PosY           float64
	LinkedDeviceID *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Link struct {
	ID         string
	GroupID    int64
	SourceID   string
	TargetID   string
	SourcePort string
	TargetPort string
	Discovered bool
	CreatedAt  time.Time
}

type Repository struct{ DB *pgxpool.Pool }

// ---- Groups ----

func (r Repository) ListGroups(ctx context.Context, tenantID string) ([]Group, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("workspacetopology repository is not initialized")
	}
	query := `SELECT id,tenant_id,name,created_at,updated_at FROM workspace_topology_groups ORDER BY created_at`
	args := []any{}
	if tenantID != "" {
		query = `SELECT id,tenant_id,name,created_at,updated_at FROM workspace_topology_groups WHERE tenant_id=$1 ORDER BY created_at`
		args = append(args, tenantID)
	}
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r Repository) CreateGroup(ctx context.Context, tenantID, name string) (Group, error) {
	if r.DB == nil {
		return Group{}, fmt.Errorf("workspacetopology repository is not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return Group{}, fmt.Errorf("name is required")
	}
	var out Group
	err := r.DB.QueryRow(ctx,
		`INSERT INTO workspace_topology_groups (tenant_id,name) VALUES ($1,$2) RETURNING id,tenant_id,name,created_at,updated_at`,
		tenantID, name).Scan(&out.ID, &out.TenantID, &out.Name, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r Repository) RenameGroup(ctx context.Context, id int64, name string) (Group, error) {
	if r.DB == nil {
		return Group{}, fmt.Errorf("workspacetopology repository is not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return Group{}, fmt.Errorf("name is required")
	}
	var out Group
	err := r.DB.QueryRow(ctx,
		`UPDATE workspace_topology_groups SET name=$2,updated_at=NOW() WHERE id=$1 RETURNING id,tenant_id,name,created_at,updated_at`,
		id, name).Scan(&out.ID, &out.TenantID, &out.Name, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

// DeleteGroup removes a group; its devices/links cascade automatically
// (ON DELETE CASCADE on group_id in both child tables).
func (r Repository) DeleteGroup(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("workspacetopology repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM workspace_topology_groups WHERE id=$1`, id)
	return err
}

// ---- Full group load (group + its devices + its links in one round trip,
// for the frontend's initial fetch when a group becomes active) ----

func (r Repository) FullGroup(ctx context.Context, groupID int64) (Group, []Device, []Link, error) {
	if r.DB == nil {
		return Group{}, nil, nil, fmt.Errorf("workspacetopology repository is not initialized")
	}
	var g Group
	err := r.DB.QueryRow(ctx, `SELECT id,tenant_id,name,created_at,updated_at FROM workspace_topology_groups WHERE id=$1`, groupID).
		Scan(&g.ID, &g.TenantID, &g.Name, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Group{}, nil, nil, err
	}
	devices, err := r.DevicesOf(ctx, groupID)
	if err != nil {
		return Group{}, nil, nil, err
	}
	links, err := r.LinksOf(ctx, groupID)
	if err != nil {
		return Group{}, nil, nil, err
	}
	return g, devices, links, nil
}

// ---- Devices ----

func (r Repository) DevicesOf(ctx context.Context, groupID int64) ([]Device, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("workspacetopology repository is not initialized")
	}
	rows, err := r.DB.Query(ctx,
		`SELECT id,group_id,name,kind,address,snmp_community,pos_x,pos_y,linked_device_id,created_at,updated_at
		 FROM workspace_topology_devices WHERE group_id=$1 ORDER BY created_at`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.GroupID, &d.Name, &d.Kind, &d.Address, &d.SNMPCommunity, &d.PosX, &d.PosY, &d.LinkedDeviceID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateDevice persists a canvas device using the id the frontend already
// generated client-side (see package doc).
func (r Repository) CreateDevice(ctx context.Context, d Device) (Device, error) {
	if r.DB == nil {
		return Device{}, fmt.Errorf("workspacetopology repository is not initialized")
	}
	if strings.TrimSpace(d.ID) == "" {
		return Device{}, fmt.Errorf("id is required")
	}
	if strings.TrimSpace(d.Name) == "" {
		return Device{}, fmt.Errorf("name is required")
	}
	var out Device
	err := r.DB.QueryRow(ctx,
		`INSERT INTO workspace_topology_devices (id,group_id,name,kind,address,snmp_community,pos_x,pos_y,linked_device_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id,group_id,name,kind,address,snmp_community,pos_x,pos_y,linked_device_id,created_at,updated_at`,
		d.ID, d.GroupID, d.Name, d.Kind, d.Address, d.SNMPCommunity, d.PosX, d.PosY, d.LinkedDeviceID).
		Scan(&out.ID, &out.GroupID, &out.Name, &out.Kind, &out.Address, &out.SNMPCommunity, &out.PosX, &out.PosY, &out.LinkedDeviceID, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

// UpdateDevice applies a full-field patch (position move, rename, or field
// edit) -- the frontend's moveDevice/updateDevice actions both funnel here.
func (r Repository) UpdateDevice(ctx context.Context, id string, d Device) (Device, error) {
	if r.DB == nil {
		return Device{}, fmt.Errorf("workspacetopology repository is not initialized")
	}
	var out Device
	err := r.DB.QueryRow(ctx,
		`UPDATE workspace_topology_devices
		 SET name=$2,kind=$3,address=$4,snmp_community=$5,pos_x=$6,pos_y=$7,linked_device_id=$8,updated_at=NOW()
		 WHERE id=$1
		 RETURNING id,group_id,name,kind,address,snmp_community,pos_x,pos_y,linked_device_id,created_at,updated_at`,
		id, d.Name, d.Kind, d.Address, d.SNMPCommunity, d.PosX, d.PosY, d.LinkedDeviceID).
		Scan(&out.ID, &out.GroupID, &out.Name, &out.Kind, &out.Address, &out.SNMPCommunity, &out.PosX, &out.PosY, &out.LinkedDeviceID, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

// DeleteDevice removes a device; its links cascade automatically (ON DELETE
// CASCADE on source_id/target_id).
func (r Repository) DeleteDevice(ctx context.Context, id string) error {
	if r.DB == nil {
		return fmt.Errorf("workspacetopology repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM workspace_topology_devices WHERE id=$1`, id)
	return err
}

// ---- Links ----

func (r Repository) LinksOf(ctx context.Context, groupID int64) ([]Link, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("workspacetopology repository is not initialized")
	}
	rows, err := r.DB.Query(ctx,
		`SELECT id,group_id,source_id,target_id,source_port,target_port,discovered,created_at
		 FROM workspace_topology_links WHERE group_id=$1 ORDER BY created_at`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.GroupID, &l.SourceID, &l.TargetID, &l.SourcePort, &l.TargetPort, &l.Discovered, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r Repository) CreateLink(ctx context.Context, l Link) (Link, error) {
	if r.DB == nil {
		return Link{}, fmt.Errorf("workspacetopology repository is not initialized")
	}
	if strings.TrimSpace(l.ID) == "" {
		return Link{}, fmt.Errorf("id is required")
	}
	if l.SourceID == l.TargetID {
		return Link{}, fmt.Errorf("sourceId and targetId must differ")
	}
	var out Link
	err := r.DB.QueryRow(ctx,
		`INSERT INTO workspace_topology_links (id,group_id,source_id,target_id,source_port,target_port,discovered)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id,group_id,source_id,target_id,source_port,target_port,discovered,created_at`,
		l.ID, l.GroupID, l.SourceID, l.TargetID, l.SourcePort, l.TargetPort, l.Discovered).
		Scan(&out.ID, &out.GroupID, &out.SourceID, &out.TargetID, &out.SourcePort, &out.TargetPort, &out.Discovered, &out.CreatedAt)
	return out, err
}

func (r Repository) DeleteLink(ctx context.Context, id string) error {
	if r.DB == nil {
		return fmt.Errorf("workspacetopology repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM workspace_topology_links WHERE id=$1`, id)
	return err
}
