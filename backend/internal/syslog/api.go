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
	ID         int64     `json:"id"`
	ReceivedAt time.Time `json:"receivedAt"`
	SourceIP   string    `json:"sourceIp"`
	Facility   *int      `json:"facility,omitempty"`
	Severity   *int      `json:"severity,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	Tag        string    `json:"tag,omitempty"`
	Message    string    `json:"message"`
}

// API backs GET /api/syslog: recent messages, optionally filtered by source
// IP and/or a maximum severity (syslog severity is inverted -- 0=emergency,
// 7=debug -- so "severity <= N" means "at least as severe as N").
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

	sql := `SELECT id,received_at,source_ip,facility,severity,COALESCE(hostname,''),COALESCE(tag,''),message FROM syslog_messages WHERE 1=1`
	args := []any{}
	if host := q.Get("host"); host != "" {
		args = append(args, host)
		sql += ` AND source_ip = $` + strconv.Itoa(len(args))
	}
	if maxSev := q.Get("maxSeverity"); maxSev != "" {
		if v, err := strconv.Atoi(maxSev); err == nil {
			args = append(args, v)
			sql += ` AND severity <= $` + strconv.Itoa(len(args))
		}
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
		if err := rows.Scan(&rec.ID, &rec.ReceivedAt, &rec.SourceIP, &facility, &severity, &rec.Hostname, &rec.Tag, &rec.Message); err != nil {
			http.Error(w, "failed to read syslog rows", http.StatusInternalServerError)
			return
		}
		rec.Facility = facility
		rec.Severity = severity
		items = append(items, rec)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
