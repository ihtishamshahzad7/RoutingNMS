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
// (metric_samples for devices, olt_alerts for OLTs, device_groups/
// device_group_members for device-group aggregation) -- statuspage does
// not duplicate monitoring logic, only presents it.
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
		case "devicegroup":
			if err := s.resolveDeviceGroup(ctx, &st); err != nil {
				return nil, err
			}
		}
		out = append(out, st)
	}
	return out, nil
}

// deviceStatus resolves a device's current status (and, if requested, its
// certificate-expiry days), independent of any particular ItemStatus --
// shared by resolveDevice (a status page's own "device" item) and
// resolveDeviceGroup (each device inside a "devicegroup" item).
func (s StatusResolver) deviceStatus(ctx context.Context, deviceID string, showCertExpiry bool) (status string, certExpiry *int, err error) {
	status = "unknown"
	var value float64
	var recordedAt time.Time
	row := s.DB.QueryRow(ctx, `SELECT value, recorded_at FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='up' ORDER BY recorded_at DESC LIMIT 1`, deviceID)
	if scanErr := row.Scan(&value, &recordedAt); scanErr == nil {
		if value == 1 {
			status = "up"
		} else {
			status = "down"
		}
	}
	if showCertExpiry {
		var days float64
		var certAt time.Time
		row := s.DB.QueryRow(ctx, `SELECT value, recorded_at FROM metric_samples WHERE subject_type='device' AND subject_id=$1 AND metric_name='http_cert_expiry_days' ORDER BY recorded_at DESC LIMIT 1`, deviceID)
		if scanErr := row.Scan(&days, &certAt); scanErr == nil {
			d := int(days)
			certExpiry = &d
		}
	}
	return status, certExpiry, nil
}

// oltStatus resolves an OLT's current status from its open olt_alerts,
// independent of any particular ItemStatus -- shared by resolveOLT and
// resolveDeviceGroup.
func (s StatusResolver) oltStatus(ctx context.Context, oltID string) (status string, err error) {
	var exists bool
	if scanErr := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM olts WHERE id=$1)`, oltID).Scan(&exists); scanErr != nil || !exists {
		return "unknown", nil
	}
	var criticalCount, warningCount int
	row := s.DB.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE severity='critical'),
		COUNT(*) FILTER (WHERE severity='warning')
		FROM olt_alerts WHERE olt_id=$1 AND status='open'`, oltID)
	if scanErr := row.Scan(&criticalCount, &warningCount); scanErr != nil {
		return "unknown", scanErr
	}
	switch {
	case criticalCount > 0:
		return "down", nil
	case warningCount > 0:
		return "degraded", nil
	default:
		return "up", nil
	}
}

func (s StatusResolver) resolveDevice(ctx context.Context, st *ItemStatus, showCertExpiry bool) error {
	var name string
	err := s.DB.QueryRow(ctx, `SELECT name FROM devices WHERE id::text=$1`, st.SubjectID).Scan(&name)
	if err == nil && st.Label == st.SubjectID {
		st.Label = name
	}
	status, certExpiry, err := s.deviceStatus(ctx, st.SubjectID, showCertExpiry)
	if err != nil {
		return err
	}
	st.Status = status
	st.CertExpiry = certExpiry
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
	status, err := s.oltStatus(ctx, st.SubjectID)
	if err != nil {
		return err
	}
	st.Status = status
	return nil
}

// statusRank orders severity for the worst-status-wins aggregation a
// device-group item shows: down is worse than degraded is worse than up;
// unknown never overrides a real reading from any member.
func statusRank(status string) int {
	switch status {
	case "down":
		return 3
	case "degraded":
		return 2
	case "up":
		return 1
	default:
		return 0
	}
}

// resolveDeviceGroup aggregates a whole device_groups group into a single
// status-page item: the group's own name becomes the label (unless
// overridden), and the status is the worst status among its device/OLT
// members (Kuma's real monitor-group page instead lists every member
// individually under a group heading; this v1 simplification shows one
// aggregate row per group -- see claude/uptime-kuma-parity.md item 32 for
// the fuller per-member breakdown as a flagged follow-up).
func (s StatusResolver) resolveDeviceGroup(ctx context.Context, st *ItemStatus) error {
	var name string
	if err := s.DB.QueryRow(ctx, `SELECT name FROM device_groups WHERE id=$1::bigint`, st.SubjectID).Scan(&name); err != nil {
		st.Status = "unknown"
		return nil
	}
	if st.Label == st.SubjectID {
		st.Label = name
	}

	rows, err := s.DB.Query(ctx, `SELECT subject_type, subject_id FROM device_group_members WHERE group_id=$1::bigint ORDER BY sort_order, id`, st.SubjectID)
	if err != nil {
		return err
	}
	defer rows.Close()
	best := "unknown"
	for rows.Next() {
		var memberType, memberID string
		if err := rows.Scan(&memberType, &memberID); err != nil {
			return err
		}
		var memberStatus string
		switch memberType {
		case "device":
			memberStatus, _, err = s.deviceStatus(ctx, memberID, false)
		case "olt":
			memberStatus, err = s.oltStatus(ctx, memberID)
		default:
			continue
		}
		if err != nil {
			return err
		}
		if statusRank(memberStatus) > statusRank(best) {
			best = memberStatus
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	st.Status = best
	return nil
}
