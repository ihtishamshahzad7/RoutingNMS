// Package statuspage implements public status pages, ported from Uptime
// Kuma's flagship feature: a branded, unauthenticated page listing chosen
// devices/OLTs and their current up/down status, for sharing with customers
// or embedding on a support site.
package statuspage

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Page struct {
	ID                    int64     `json:"id"`
	TenantID              string    `json:"tenantId"`
	Slug                  string    `json:"slug"`
	Title                 string    `json:"title"`
	Description           string    `json:"description"`
	Published             bool      `json:"published"`
	ShowCertificateExpiry bool      `json:"showCertificateExpiry"`
	FooterText            string    `json:"footerText"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type Item struct {
	ID           int64  `json:"id"`
	StatusPageID int64  `json:"statusPageId"`
	SubjectType  string `json:"subjectType"` // "device" | "olt"
	SubjectID    string `json:"subjectId"`
	Label        string `json:"label"`
	Position     int    `json:"position"`
}

type Repository struct{ DB *pgxpool.Pool }

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug enforces a URL-safe slug -- this becomes the public path
// (/status/{slug}), so it must never contain characters that would need
// escaping or could be confused with another route.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("slug must be lowercase letters, numbers and hyphens only (e.g. \"network-status\")")
	}
	return nil
}

func (r Repository) List(ctx context.Context, tenantID string) ([]Page, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("statuspage repository is not initialized")
	}
	query := `SELECT id,tenant_id,slug,title,description,published,show_certificate_expiry,footer_text,created_at,updated_at FROM status_pages ORDER BY title`
	args := []any{}
	if tenantID != "" {
		query = `SELECT id,tenant_id,slug,title,description,published,show_certificate_expiry,footer_text,created_at,updated_at FROM status_pages WHERE tenant_id=$1 ORDER BY title`
		args = append(args, tenantID)
	}
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Page{}
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Slug, &p.Title, &p.Description, &p.Published, &p.ShowCertificateExpiry, &p.FooterText, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r Repository) Get(ctx context.Context, id int64) (Page, error) {
	if r.DB == nil {
		return Page{}, fmt.Errorf("statuspage repository is not initialized")
	}
	var p Page
	err := r.DB.QueryRow(ctx, `SELECT id,tenant_id,slug,title,description,published,show_certificate_expiry,footer_text,created_at,updated_at FROM status_pages WHERE id=$1`, id).
		Scan(&p.ID, &p.TenantID, &p.Slug, &p.Title, &p.Description, &p.Published, &p.ShowCertificateExpiry, &p.FooterText, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r Repository) GetBySlug(ctx context.Context, slug string) (Page, error) {
	if r.DB == nil {
		return Page{}, fmt.Errorf("statuspage repository is not initialized")
	}
	var p Page
	err := r.DB.QueryRow(ctx, `SELECT id,tenant_id,slug,title,description,published,show_certificate_expiry,footer_text,created_at,updated_at FROM status_pages WHERE slug=$1`, slug).
		Scan(&p.ID, &p.TenantID, &p.Slug, &p.Title, &p.Description, &p.Published, &p.ShowCertificateExpiry, &p.FooterText, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r Repository) Create(ctx context.Context, p Page) (Page, error) {
	if r.DB == nil {
		return Page{}, fmt.Errorf("statuspage repository is not initialized")
	}
	if strings.TrimSpace(p.Title) == "" {
		return Page{}, fmt.Errorf("title is required")
	}
	if err := ValidateSlug(p.Slug); err != nil {
		return Page{}, err
	}
	var out Page
	err := r.DB.QueryRow(ctx, `INSERT INTO status_pages (tenant_id,slug,title,description,published,show_certificate_expiry,footer_text) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id,tenant_id,slug,title,description,published,show_certificate_expiry,footer_text,created_at,updated_at`,
		p.TenantID, p.Slug, p.Title, p.Description, p.Published, p.ShowCertificateExpiry, p.FooterText).
		Scan(&out.ID, &out.TenantID, &out.Slug, &out.Title, &out.Description, &out.Published, &out.ShowCertificateExpiry, &out.FooterText, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r Repository) Update(ctx context.Context, id int64, p Page) (Page, error) {
	if r.DB == nil {
		return Page{}, fmt.Errorf("statuspage repository is not initialized")
	}
	if err := ValidateSlug(p.Slug); err != nil {
		return Page{}, err
	}
	var out Page
	err := r.DB.QueryRow(ctx, `UPDATE status_pages SET slug=$2,title=$3,description=$4,published=$5,show_certificate_expiry=$6,footer_text=$7,updated_at=NOW() WHERE id=$1 RETURNING id,tenant_id,slug,title,description,published,show_certificate_expiry,footer_text,created_at,updated_at`,
		id, p.Slug, p.Title, p.Description, p.Published, p.ShowCertificateExpiry, p.FooterText).
		Scan(&out.ID, &out.TenantID, &out.Slug, &out.Title, &out.Description, &out.Published, &out.ShowCertificateExpiry, &out.FooterText, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("statuspage repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM status_pages WHERE id=$1`, id)
	return err
}

func (r Repository) ListItems(ctx context.Context, pageID int64) ([]Item, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("statuspage repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,status_page_id,subject_type,subject_id,label,position FROM status_page_items WHERE status_page_id=$1 ORDER BY position, id`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.StatusPageID, &it.SubjectType, &it.SubjectID, &it.Label, &it.Position); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReplaceItems atomically replaces the full item list for a page -- the
// admin UI always submits the complete ordered list, not incremental
// add/remove operations.
func (r Repository) ReplaceItems(ctx context.Context, pageID int64, items []Item) error {
	if r.DB == nil {
		return fmt.Errorf("statuspage repository is not initialized")
	}
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM status_page_items WHERE status_page_id=$1`, pageID); err != nil {
		return err
	}
	for i, it := range items {
		if it.SubjectType != "device" && it.SubjectType != "olt" {
			return fmt.Errorf("item %d: subjectType must be \"device\" or \"olt\"", i)
		}
		if strings.TrimSpace(it.SubjectID) == "" {
			return fmt.Errorf("item %d: subjectId is required", i)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO status_page_items (status_page_id,subject_type,subject_id,label,position) VALUES ($1,$2,$3,$4,$5)`,
			pageID, it.SubjectType, it.SubjectID, it.Label, i); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
