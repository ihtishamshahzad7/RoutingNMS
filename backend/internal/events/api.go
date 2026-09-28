package events

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// Record is the JSON shape returned by GET /api/v1/events.
type Record struct {
	ID         int64     `json:"id"`
	DeviceID   *int64    `json:"deviceId,omitempty"`
	GroupID    *int64    `json:"groupId,omitempty"`
	EventType  string    `json:"eventType"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Source     string    `json:"source"`
	RefTable   string    `json:"refTable,omitempty"`
	RefID      string    `json:"refId,omitempty"`
	Count      int       `json:"count"`
	FirstSeen  time.Time `json:"firstSeen"`
	LastSeen   time.Time `json:"lastSeen"`
}

// listResponse wraps the page of results with pagination metadata, so the
// frontend (a later sub-item) knows whether there's another page without
// a second request.
type listResponse struct {
	Items      []Record `json:"items"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
	HasMore    bool     `json:"hasMore"`
}

// API backs GET /api/v1/events: the unified event log, filterable by time
// range, severity, device, group, event type, and free text, with
// pagination.
//
// Query params: from, to (RFC3339), severity, device (device id), group
// (group id), eventType, text (ILIKE substring over message), limit
// (default 100, max 1000), offset (default 0).
type API struct{ Repo Repository }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.Repo.DB == nil {
		http.Error(w, "database is not initialized", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()

	limit := 100
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 1000 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	sql := `SELECT id, device_id, group_id, event_type, severity, message, source,
			ref_table, ref_id, count, first_seen, last_seen
		FROM events WHERE 1=1`
	args := []any{}

	if from := q.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			args = append(args, t)
			sql += ` AND last_seen >= $` + strconv.Itoa(len(args))
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			args = append(args, t)
			sql += ` AND first_seen <= $` + strconv.Itoa(len(args))
		}
	}
	if severity := q.Get("severity"); severity != "" {
		args = append(args, severity)
		sql += ` AND severity = $` + strconv.Itoa(len(args))
	}
	if device := q.Get("device"); device != "" {
		if id, err := strconv.ParseInt(device, 10, 64); err == nil {
			args = append(args, id)
			sql += ` AND device_id = $` + strconv.Itoa(len(args))
		}
	}
	if group := q.Get("group"); group != "" {
		if id, err := strconv.ParseInt(group, 10, 64); err == nil {
			args = append(args, id)
			sql += ` AND group_id = $` + strconv.Itoa(len(args))
		}
	}
	if eventType := q.Get("eventType"); eventType != "" {
		args = append(args, eventType)
		sql += ` AND event_type = $` + strconv.Itoa(len(args))
	}
	if text := q.Get("text"); text != "" {
		args = append(args, "%"+text+"%")
		sql += ` AND message ILIKE $` + strconv.Itoa(len(args))
	}

	sql += ` ORDER BY last_seen DESC`
	// Fetch one extra row to determine hasMore without a second COUNT(*) query.
	args = append(args, limit+1)
	sql += ` LIMIT $` + strconv.Itoa(len(args))
	args = append(args, offset)
	sql += ` OFFSET $` + strconv.Itoa(len(args))

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := a.Repo.DB.Query(ctx, sql, args...)
	if err != nil {
		http.Error(w, "failed to query events", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []Record{}
	for rows.Next() {
		var rec Record
		var deviceID, groupID *int64
		if err := rows.Scan(&rec.ID, &deviceID, &groupID, &rec.EventType, &rec.Severity, &rec.Message,
			&rec.Source, &rec.RefTable, &rec.RefID, &rec.Count, &rec.FirstSeen, &rec.LastSeen); err != nil {
			http.Error(w, "failed to read events rows", http.StatusInternalServerError)
			return
		}
		rec.DeviceID = deviceID
		rec.GroupID = groupID
		items = append(items, rec)
	}

	hasMore := false
	if len(items) > limit {
		items = items[:limit]
		hasMore = true
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(listResponse{Items: items, Limit: limit, Offset: offset, HasMore: hasMore})
}
