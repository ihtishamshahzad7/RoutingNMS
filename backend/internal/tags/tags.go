// Package tags implements free-form, colored tags on devices/OLTs, ported
// from Uptime Kuma (any monitor there can carry arbitrary tags used to
// filter/organize the dashboard). Distinct from RoutingNMS's ISP-specific
// hierarchy (sites/access-points/customers), which Kuma has no equivalent
// of -- tags are for ad-hoc, cross-cutting labels ("core", "customer-edge",
// "needs-firmware") that don't fit that hierarchy.
package tags

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Tag struct {
	ID       int64  `json:"id"`
	TenantID string `json:"tenantId"`
	Name     string `json:"name"`
	Color    string `json:"color"`
}

// Assignment links one tag to one subject (a device or an OLT).
type Assignment struct {
	TagID       int64  `json:"tagId"`
	SubjectType string `json:"subjectType"` // "device" | "olt"
	SubjectID   string `json:"subjectId"`
}

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) List(ctx context.Context, tenantID string) ([]Tag, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("tags repository is not initialized")
	}
	query := `SELECT id,tenant_id,name,color FROM tags ORDER BY name`
	args := []any{}
	if tenantID != "" {
		query = `SELECT id,tenant_id,name,color FROM tags WHERE tenant_id=$1 ORDER BY name`
		args = append(args, tenantID)
	}
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r Repository) Create(ctx context.Context, t Tag) (Tag, error) {
	if r.DB == nil {
		return Tag{}, fmt.Errorf("tags repository is not initialized")
	}
	if strings.TrimSpace(t.Name) == "" {
		return Tag{}, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(t.Color) == "" {
		t.Color = "#58A6FF"
	}
	var out Tag
	err := r.DB.QueryRow(ctx, `INSERT INTO tags (tenant_id,name,color) VALUES ($1,$2,$3) RETURNING id,tenant_id,name,color`,
		t.TenantID, t.Name, t.Color).Scan(&out.ID, &out.TenantID, &out.Name, &out.Color)
	return out, err
}

func (r Repository) Update(ctx context.Context, id int64, t Tag) (Tag, error) {
	if r.DB == nil {
		return Tag{}, fmt.Errorf("tags repository is not initialized")
	}
	if strings.TrimSpace(t.Name) == "" {
		return Tag{}, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(t.Color) == "" {
		t.Color = "#58A6FF"
	}
	var out Tag
	err := r.DB.QueryRow(ctx, `UPDATE tags SET name=$2,color=$3 WHERE id=$1 RETURNING id,tenant_id,name,color`,
		id, t.Name, t.Color).Scan(&out.ID, &out.TenantID, &out.Name, &out.Color)
	return out, err
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("tags repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM tags WHERE id=$1`, id)
	return err
}

// ForSubject lists the tags currently assigned to one device/OLT.
func (r Repository) ForSubject(ctx context.Context, subjectType, subjectID string) ([]Tag, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("tags repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `
		SELECT t.id, t.tenant_id, t.name, t.color
		FROM tags t
		JOIN tag_assignments a ON a.tag_id = t.id
		WHERE a.subject_type = $1 AND a.subject_id = $2
		ORDER BY t.name`, subjectType, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AllAssignments returns every (tagId, subjectType, subjectId) assignment --
// used by the devices/OLTs list views to annotate each row with its tags in
// one query instead of one-per-row.
func (r Repository) AllAssignments(ctx context.Context) ([]Assignment, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("tags repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT tag_id, subject_type, subject_id FROM tag_assignments`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Assignment{}
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.TagID, &a.SubjectType, &a.SubjectID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ReplaceForSubject atomically sets the full tag list for one device/OLT --
// the UI always submits the complete set (a multi-select), not incremental
// add/remove operations.
func (r Repository) ReplaceForSubject(ctx context.Context, subjectType, subjectID string, tagIDs []int64) error {
	if r.DB == nil {
		return fmt.Errorf("tags repository is not initialized")
	}
	if subjectType != "device" && subjectType != "olt" {
		return fmt.Errorf("subjectType must be \"device\" or \"olt\"")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM tag_assignments WHERE subject_type=$1 AND subject_id=$2`, subjectType, subjectID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO tag_assignments (tag_id,subject_type,subject_id) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			tagID, subjectType, subjectID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
