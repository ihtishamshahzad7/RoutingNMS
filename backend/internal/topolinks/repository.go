package topolinks

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) CreateGroup(ctx context.Context, organizationID, name string) (Group, error) {
	if r.DB == nil {
		return Group{}, fmt.Errorf("topolinks repository is not initialized")
	}
	var g Group
	err := r.DB.QueryRow(ctx, `INSERT INTO topo_link_groups (organization_id,name) VALUES ($1,$2) RETURNING id,organization_id,name,created_at`,
		organizationID, name).Scan(&g.ID, &g.OrganizationID, &g.Name, &g.CreatedAt)
	return g, err
}

func (r Repository) ListGroups(ctx context.Context, organizationID string) ([]Group, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("topolinks repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,organization_id,name,created_at FROM topo_link_groups WHERE organization_id=$1 ORDER BY created_at`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.OrganizationID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r Repository) DeleteGroup(ctx context.Context, id string) error {
	if r.DB == nil {
		return fmt.Errorf("topolinks repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM topo_link_groups WHERE id=$1`, id)
	return err
}

func (r Repository) CreateLink(ctx context.Context, groupID, deviceAID, interfaceA, deviceBID, interfaceB string) (Link, error) {
	if r.DB == nil {
		return Link{}, fmt.Errorf("topolinks repository is not initialized")
	}
	var l Link
	err := r.DB.QueryRow(ctx, `INSERT INTO topo_links (group_id,device_a_id,interface_a,device_b_id,interface_b) VALUES ($1,$2,$3,$4,$5)
		RETURNING id,group_id,device_a_id,interface_a,device_b_id,interface_b,created_at`,
		groupID, deviceAID, interfaceA, deviceBID, interfaceB).
		Scan(&l.ID, &l.GroupID, &l.DeviceAID, &l.InterfaceA, &l.DeviceBID, &l.InterfaceB, &l.CreatedAt)
	return l, err
}

// ListLinks returns every link in a group, joined with device names for
// display.
func (r Repository) ListLinks(ctx context.Context, groupID string) ([]Link, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("topolinks repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT l.id,l.group_id,l.device_a_id,da.name,l.interface_a,l.device_b_id,db.name,l.interface_b,l.created_at
		FROM topo_links l
		JOIN devices da ON da.id = l.device_a_id
		JOIN devices db ON db.id = l.device_b_id
		WHERE l.group_id=$1 ORDER BY l.created_at`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.GroupID, &l.DeviceAID, &l.DeviceAName, &l.InterfaceA, &l.DeviceBID, &l.DeviceBName, &l.InterfaceB, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ListAll returns every link across every group, for the background poller.
func (r Repository) ListAll(ctx context.Context) ([]Link, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("topolinks repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT l.id,l.group_id,l.device_a_id,da.name,l.interface_a,l.device_b_id,db.name,l.interface_b,l.created_at
		FROM topo_links l
		JOIN devices da ON da.id = l.device_a_id
		JOIN devices db ON db.id = l.device_b_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.GroupID, &l.DeviceAID, &l.DeviceAName, &l.InterfaceA, &l.DeviceBID, &l.DeviceBName, &l.InterfaceB, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r Repository) DeleteLink(ctx context.Context, id string) error {
	if r.DB == nil {
		return fmt.Errorf("topolinks repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM topo_links WHERE id=$1`, id)
	return err
}

// GroupName returns a group's name (used to build "group X device Y port Z
// down" alert messages without a second round trip per link).
func (r Repository) GroupName(ctx context.Context, id string) (string, error) {
	if r.DB == nil {
		return "", fmt.Errorf("topolinks repository is not initialized")
	}
	var name string
	err := r.DB.QueryRow(ctx, `SELECT name FROM topo_link_groups WHERE id=$1`, id).Scan(&name)
	return name, err
}
