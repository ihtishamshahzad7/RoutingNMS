package mib

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

type StoredMIB struct {
	ID           int64     `json:"id"`
	Filename     string    `json:"filename"`
	ModuleName   string    `json:"moduleName,omitempty"`
	ObjectCount  int       `json:"objectCount"`
	SkippedCount int       `json:"skippedCount"`
	UploadedAt   time.Time `json:"uploadedAt"`
}

// Upload parses rawText and persists both the MIB record and its resolved
// name<->OID objects in a single transaction.
func (r Repository) Upload(ctx context.Context, filename, rawText string) (StoredMIB, error) {
	if r.DB == nil {
		return StoredMIB{}, fmt.Errorf("mib repository is not initialized")
	}
	result := Parse(rawText)

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return StoredMIB{}, err
	}
	defer tx.Rollback(ctx)

	var st StoredMIB
	err = tx.QueryRow(ctx, `INSERT INTO mibs (filename,module_name,raw_text,object_count,skipped_count) VALUES ($1,$2,$3,$4,$5) RETURNING id,uploaded_at`,
		filename, nullableStr(result.ModuleName), rawText, len(result.Objects), result.Skipped).Scan(&st.ID, &st.UploadedAt)
	if err != nil {
		return StoredMIB{}, err
	}

	for _, obj := range result.Objects {
		if _, err := tx.Exec(ctx, `INSERT INTO mib_objects (mib_id,name,oid) VALUES ($1,$2,$3)`, st.ID, obj.Name, obj.OID); err != nil {
			return StoredMIB{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return StoredMIB{}, err
	}

	st.Filename = filename
	st.ModuleName = result.ModuleName
	st.ObjectCount = len(result.Objects)
	st.SkippedCount = result.Skipped
	return st, nil
}

func (r Repository) List(ctx context.Context) ([]StoredMIB, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("mib repository is not initialized")
	}
	rows, err := r.DB.Query(ctx, `SELECT id,filename,COALESCE(module_name,''),object_count,skipped_count,uploaded_at FROM mibs ORDER BY uploaded_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoredMIB{}
	for rows.Next() {
		var st StoredMIB
		if err := rows.Scan(&st.ID, &st.Filename, &st.ModuleName, &st.ObjectCount, &st.SkippedCount, &st.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (r Repository) Delete(ctx context.Context, id int64) error {
	if r.DB == nil {
		return fmt.Errorf("mib repository is not initialized")
	}
	_, err := r.DB.Exec(ctx, `DELETE FROM mibs WHERE id=$1`, id)
	return err
}

// SearchResult is one name<->OID hit, tagged with which MIB it came from.
type SearchResult struct {
	Name     string `json:"name"`
	OID      string `json:"oid"`
	MIBID    int64  `json:"mibId"`
	Filename string `json:"filename"`
}

// Search matches q as a case-insensitive substring of the object name, or
// an exact/prefix match against the OID -- covering both "what's the OID
// for sysDescr" and "what is 1.3.6.1.2.1.1.1" lookups from one box.
func (r Repository) Search(ctx context.Context, q string, limit int) ([]SearchResult, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("mib repository is not initialized")
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return []SearchResult{}, nil
	}
	rows, err := r.DB.Query(ctx, `
		SELECT o.name, o.oid, o.mib_id, m.filename
		FROM mib_objects o
		JOIN mibs m ON m.id = o.mib_id
		WHERE lower(o.name) LIKE lower($1) OR o.oid = $2 OR o.oid LIKE $3
		ORDER BY length(o.name) ASC
		LIMIT $4`,
		"%"+q+"%", q, q+".%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchResult{}
	for rows.Next() {
		var sr SearchResult
		if err := rows.Scan(&sr.Name, &sr.OID, &sr.MIBID, &sr.Filename); err != nil {
			return nil, err
		}
		out = append(out, sr)
	}
	return out, rows.Err()
}

// ResolveName returns the friendliest known name for oid (exact match
// first, then the longest known ancestor OID + the remaining suffix, e.g.
// "ifDescr.3" for "1.3.6.1.2.1.2.2.1.2.3"), or "" if nothing matches.
func (r Repository) ResolveName(ctx context.Context, oid string) (string, error) {
	if r.DB == nil {
		return "", fmt.Errorf("mib repository is not initialized")
	}
	var name string
	err := r.DB.QueryRow(ctx, `SELECT name FROM mib_objects WHERE oid = $1 LIMIT 1`, oid).Scan(&name)
	if err == nil {
		return name, nil
	}
	rows, err := r.DB.Query(ctx, `SELECT name, oid FROM mib_objects WHERE $1 LIKE oid || '.%' ORDER BY length(oid) DESC LIMIT 1`, oid)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if rows.Next() {
		var n, ancestorOID string
		if err := rows.Scan(&n, &ancestorOID); err != nil {
			return "", err
		}
		suffix := strings.TrimPrefix(oid, ancestorOID+".")
		return n + "." + suffix, nil
	}
	return "", nil
}

func nullableStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}
