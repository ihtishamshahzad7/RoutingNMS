package provisioning

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Template struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	ScriptBody string    `json:"scriptBody"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) List(ctx context.Context) ([]Template, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("provisioning repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,name,script_body,created_at,updated_at FROM provisioning_templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Template{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.Name, &t.ScriptBody, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r Repository) Get(ctx context.Context, id int64) (Template, error) {
	if r.DB == nil {
		return Template{}, fmt.Errorf("provisioning repository is not initialized")
	}
	var t Template
	err := r.DB.QueryRow(ctx, `SELECT id,name,script_body,created_at,updated_at FROM provisioning_templates WHERE id=$1`, id).
		Scan(&t.ID, &t.Name, &t.ScriptBody, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r Repository) Create(ctx context.Context, name, scriptBody string) (Template, error) {
	if r.DB == nil {
		return Template{}, fmt.Errorf("provisioning repository is not initialized")
	}
	if strings.TrimSpace(name) == "" {
		return Template{}, fmt.Errorf("template name is required")
	}
	if strings.TrimSpace(scriptBody) == "" {
		return Template{}, fmt.Errorf("script body is required")
	}
	if _, err := template.New("check").Parse(scriptBody); err != nil {
		return Template{}, fmt.Errorf("script body is not a valid template: %w", err)
	}
	var t Template
	err := r.DB.QueryRow(ctx, `INSERT INTO provisioning_templates (name,script_body) VALUES ($1,$2) RETURNING id,name,script_body,created_at,updated_at`, name, scriptBody).
		Scan(&t.ID, &t.Name, &t.ScriptBody, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r Repository) Update(ctx context.Context, id int64, name, scriptBody string) (Template, error) {
	if r.DB == nil {
		return Template{}, fmt.Errorf("provisioning repository is not initialized")
	}
	if _, err := template.New("check").Parse(scriptBody); err != nil {
		return Template{}, fmt.Errorf("script body is not a valid template: %w", err)
	}
	var t Template
	err := r.DB.QueryRow(ctx, `UPDATE provisioning_templates SET name=$2, script_body=$3, updated_at=NOW() WHERE id=$1 RETURNING id,name,script_body,created_at,updated_at`, id, name, scriptBody).
		Scan(&t.ID, &t.Name, &t.ScriptBody, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("provisioning repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM provisioning_templates WHERE id=$1`, id)
	return err
}

// RenderData is what a provisioning template can reference.
type RenderData struct {
	Hostname     string
	Address      string
	Password     string
	SerialNumber string
	Model        string
}

// Render executes a template's script body against a device's data.
func Render(scriptBody string, data RenderData) (string, error) {
	tpl, err := template.New("script").Parse(scriptBody)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DerivePassword deterministically derives a per-device admin password from
// its serial number and a server-side salt via HMAC-SHA256, so the password
// is never stored -- it's recomputed identically every time the same
// serial+salt pair is asked for, but is not guessable without the salt.
func DerivePassword(serial, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(strings.ToUpper(strings.TrimSpace(serial))))
	sum := hex.EncodeToString(mac.Sum(nil))
	if len(sum) > 20 {
		sum = sum[:20]
	}
	return "Rn-" + sum
}
