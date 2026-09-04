package statuspage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ItemStatus is one item's current computed status, ready for the public
// page to render.
type ItemStatus struct {
	SubjectType string     `json:"subjectType"`
	SubjectID   string     `json:"subjectId"`
	Label       string     `json:"label"`
	Status      string     `json:"status"` // "up" | "down" | "degraded" | "unknown"
	CertExpiry  *int       `json:"certExpiryDays,omitempty"`
	Since       *time.Time `json:"since,omitempty"`
}

// StatusResolver computes the current up/down status for status-page items.
// It's a thin read-only view over data other packages already own
// (metric_samples for devices, olt_alerts for OLTs) -- statuspage does not
// duplicate monitoring logic, only presents it.
type StatusResolver struct{ DB *pgxpool.Pool }

func (s StatusResolver) Resolve(ctx context.Context, items []Item, showCertExpiry bool) ([]ItemStatus, error) {
	out := make([]ItemStatus, 0, len(items))
	for _, it := range items {
		st := ItemStatus{SubjectType: it.SubjectType, SubjectID: it.SubjectID, Label: it.Label, Status: "unknown"}
		if st.Label == "" {
			st.Label = it.SubjectID
		}
		switch it.SubjectType {
		case "device":
			if err := s.resolveDevice(ctx, &st, showCertExpiry); err != nil {
				return nil, err
			}
		case "olt":
			if err := s.resolveOLT(ctx, &st); err != nil {
				return nil, err
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func (s StatusResolver) resolveDevice(ctx context.Context, st *ItemStatus, showCertExpiry bool) error {
	var name string
	err := s.DB.QueryRow(ctx, `SELECT name FROM devices WHERE id::text=$1`, st.SubjectID).Scan(&name)
	if err == nil && st.Label == st.SubjectID {
		st.Label = name
	}

	var value float64
	var recordedAt time.Time
	row := s.DB.QueryRow(ctx, `SELECT value, recorded_at FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='up' ORDER BY recorded_at DESC LIMIT 1`, st.SubjectID)
	if err := row.Scan(&value, &recordedAt); err == nil {
		if value == 1 {
			st.Status = "up"
		} else {
			st.Status = "down"
		}
		st.Since = &recordedAt
	}

	if showCertExpiry {
		var days float64
		var certAt time.Time
		row := s.DB.QueryRow(ctx, `SELECT value, recorded_at FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='http_cert_expiry_days' ORDER BY recorded_at DESC LIMIT 1`, st.SubjectID)
		if err := row.Scan(&days, &certAt); err == nil {
			d := int(days)
			st.CertExpiry = &d
		}
	}
	return nil
}

func (s StatusResolver) resolveOLT(ctx context.Context, st *ItemStatus) error {
	var name string
	err := s.DB.QueryRow(ctx, `SELECT name FROM olts WHERE id=$1`, st.SubjectID).Scan(&name)
	if err != nil {
		st.Status = "unknown"
		return nil
	}
	if st.Label == st.SubjectID {
		st.Label = name
	}

	var criticalCount, warningCount int
	row := s.DB.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE severity='critical'),
		COUNT(*) FILTER (WHERE severity='warning')
		FROM olt_alerts WHERE olt_id=$1 AND status='open'`, st.SubjectID)
	if err := row.Scan(&criticalCount, &warningCount); err != nil {
		return err
	}
	switch {
	case criticalCount > 0:
		st.Status = "down"
	case warningCount > 0:
		st.Status = "degraded"
	default:
		st.Status = "up"
	}
	return nil
}
