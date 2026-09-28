package syslog

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	ID                 int64      `json:"id"`
	ReceivedAt         time.Time  `json:"receivedAt"`
	SourceIP           string     `json:"sourceIp"`
	Facility           *int       `json:"facility,omitempty"`
	Severity           *int       `json:"severity,omitempty"`
	Hostname           string     `json:"hostname,omitempty"`
	Tag                string     `json:"tag,omitempty"`
	Message            string     `json:"message"`
	// Item 3.2 additions. DeviceID is nil ("unknown device") when the
	// packet's source IP didn't match any configured device address.
	DeviceID            *string    `json:"deviceId,omitempty"`
	DeviceKnown         bool       `json:"deviceKnown"`
	MessageTimestamp    time.Time  `json:"messageTimestamp"`
	TimestampEstimated  bool       `json:"timestampEstimated"`
}

// API backs GET /api/v1/syslog: the latest messages, filterable by device,
// severity, and free-text search over the message body -- plus the
// original host/maxSeverity filters, kept for backward compatibility.
type API struct{ DB *pgxpool.Pool }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.DB == nil {
		http.Error(w, "database is not initialized", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	limit := 200
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 1000 {
		limit = v
	}

	sql := `SELECT id,received_at,source_ip,facility,severity,COALESCE(hostname,''),COALESCE(tag,''),message,
			device_id,message_timestamp,timestamp_estimated
		FROM syslog_messages WHERE 1=1`
	args := []any{}

	// host: exact source IP match (original filter, kept as-is).
	if host := q.Get("host"); host != "" {
		args = append(args, host)
		sql += ` AND source_ip = $` + strconv.Itoa(len(args))
	}
	// device: numeric device ID for an exact match, or the literal string
	// "unknown" to list only messages that couldn't be matched to a device.
	if device := q.Get("device"); device != "" {
		if device == "unknown" {
			sql += ` AND device_id IS NULL`
		} else if id, err := strconv.ParseInt(device, 10, 64); err == nil {
			args = append(args, id)
			sql += ` AND device_id = $` + strconv.Itoa(len(args))
		}
	}
	// severity: exact match (0=emergency .. 7=debug, per RFC 5424 6.2.1).
	if sev := q.Get("severity"); sev != "" {
		if v, err := strconv.Atoi(sev); err == nil {
			args = append(args, v)
			sql += ` AND severity = $` + strconv.Itoa(len(args))
		}
	}
	// maxSeverity: kept for backward compatibility -- "at least as severe
	// as N" (syslog severity is inverted: lower number = more severe).
	if maxSev := q.Get("maxSeverity"); maxSev != "" {
		if v, err := strconv.Atoi(maxSev); err == nil {
			args = append(args, v)
			sql += ` AND severity <= $` + strconv.Itoa(len(args))
		}
	}
	// text: case-insensitive substring search over the message body.
	if text := q.Get("text"); text != "" {
		args = append(args, "%"+text+"%")
		sql += ` AND message ILIKE $` + strconv.Itoa(len(args))
	}

	args = append(args, limit)
	sql += ` ORDER BY received_at DESC LIMIT $` + strconv.Itoa(len(args))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.DB.Query(ctx, sql, args...)
	if err != nil {
		http.Error(w, "failed to query syslog", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []Record{}
	for rows.Next() {
		var rec Record
		var facility, severity *int
		var deviceID *int64
		if err := rows.Scan(&rec.ID, &rec.ReceivedAt, &rec.SourceIP, &facility, &severity, &rec.Hostname, &rec.Tag, &rec.Message,
			&deviceID, &rec.MessageTimestamp, &rec.TimestampEstimated); err != nil {
			http.Error(w, "failed to read syslog rows", http.StatusInternalServerError)
			return
		}
		rec.Facility = facility
		rec.Severity = severity
		if deviceID != nil {
			s := strconv.FormatInt(*deviceID, 10)
			rec.DeviceID = &s
			rec.DeviceKnown = true
		}
		items = append(items, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
