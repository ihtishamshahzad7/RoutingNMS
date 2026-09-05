// Package devicegroups implements a lightweight, named-folder concept for
// organizing devices/OLTs, ported from Uptime Kuma's per-status-page monitor
// grouping (a simple join table with a weight/order column) -- NOT Kuma's
// other, more invasive grouping mechanism where a "group" is itself a
// special monitor type with parent/child cascading active-state.
//
// Distinct from tags (free-form, cross-cutting, many-to-many labels): a
// group is an exclusive-ish organizational folder -- in this v1, a device
// belongs to zero or one group.
package devicegroups

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID        int64  `json:"id"`
	TenantID  string `json:"tenantId"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

// Member links one group to one subject (a device or an OLT).
type Member struct {
	GroupID     int64  `json:"groupId"`
	SubjectType string `json:"subjectType"` // "device" | "olt"
	SubjectID   string `json:"subjectId"`
	SortOrder   int    `json:"sortOrder"`
}

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) List(ctx context.Context, tenantID string) ([]Group, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("devicegroups repository is not initialized")
	}
	query := `SELECT id,tenant_id,name,sort_order FROM device_groups ORDER BY sort_order, name`
	args := []any{}
	if tenantID != "" {
		query = `SELECT id,tenant_id,name,sort_order FROM device_groups WHERE tenant_id=$1 ORDER BY sort_order, name`
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
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r Repository) Create(ctx context.Context, g Group) (Group, error) {
	if r.DB == nil {
		return Group{}, fmt.Errorf("devicegroups repository is not initialized")
	}
	if strings.TrimSpace(g.Name) == "" {
		return Group{}, fmt.Errorf("name is required")
	}
	var out Group
	err := r.DB.QueryRow(ctx, `INSERT INTO device_groups (tenant_id,name,sort_order) VALUES ($1,$2,$3) RETURNING id,tenant_id,name,sort_order`,
		g.TenantID, g.Name, g.SortOrder).Scan(&out.ID, &out.TenantID, &out.Name, &out.SortOrder)
	return out, err
}

func (r Repository) Update(ctx context.Context, id int64, g Group) (Group, error) {
	if r.DB == nil {
		return Group{}, fmt.Errorf("devicegroups repository is not initialized")
	}
	if strings.TrimSpace(g.Name) == "" {
		return Group{}, fmt.Errorf("name is required")
	}
	var out Group
	err := r.DB.QueryRow(ctx, `UPDATE device_groups SET name=$2,sort_order=$3,updated_at=NOW() WHERE id=$1 RETURNING id,tenant_id,name,sort_order`,
		id, g.Name, g.SortOrder).Scan(&out.ID, &out.TenantID, &out.Name, &out.SortOrder)
	return out, err
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("devicegroups repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM device_groups WHERE id=$1`, id)
	return err
}

// MembersOf lists the devices/OLTs currently assigned to one group, ordered.
func (r Repository) MembersOf(ctx context.Context, groupID int64) ([]Member, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("devicegroups repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT group_id,subject_type,subject_id,sort_order FROM device_group_members WHERE group_id=$1 ORDER BY sort_order, id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.GroupID, &m.SubjectType, &m.SubjectID, &m.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AllMembers returns every (groupId, subjectType, subjectId, sortOrder)
// membership -- used by the devices/OLTs list views to annotate each row
// with its group in one query instead of one-per-row.
func (r Repository) AllMembers(ctx context.Context) ([]Member, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("devicegroups repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT group_id,subject_type,subject_id,sort_order FROM device_group_members ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.GroupID, &m.SubjectType, &m.SubjectID, &m.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetForSubject assigns one device/OLT to a group (replacing any prior
// membership, since a subject belongs to at most one group), or clears its
// group entirely when groupID is nil.
func (r Repository) SetForSubject(ctx context.Context, subjectType, subjectID string, groupID *int64, sortOrder int) error {
	if r.DB == nil {
		return fmt.Errorf("devicegroups repository is not initialized")
	}
	if subjectType != "device" && subjectType != "olt" {
		return fmt.Errorf("subjectType must be \"device\" or \"olt\"")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM device_group_members WHERE subject_type=$1 AND subject_id=$2`, subjectType, subjectID); err != nil {
		return err
	}
	if groupID != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO device_group_members (group_id,subject_type,subject_id,sort_order) VALUES ($1,$2,$3,$4)`,
			*groupID, subjectType, subjectID, sortOrder); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Reorder sets the sort_order of every member of one group, in the order
// given -- the UI submits the complete ordered subject list for the group.
func (r Repository) Reorder(ctx context.Context, groupID int64, subjectIDs []string) error {
	if r.DB == nil {
		return fmt.Errorf("devicegroups repository is not initialized")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for i, subjectID := range subjectIDs {
		if _, err := tx.Exec(ctx, `UPDATE device_group_members SET sort_order=$1 WHERE group_id=$2 AND subject_id=$3`, i, groupID, subjectID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
