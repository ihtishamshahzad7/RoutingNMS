package olt

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertAPI struct { DB *pgxpool.Pool }

func (a AlertAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if a.DB == nil { http.Error(w, "database is not initialized", http.StatusServiceUnavailable); return }
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/olts/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "alerts" { http.NotFound(w, r); return }
	oltID := parts[0]
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 { http.Error(w, "limit must be between 1 and 200", http.StatusBadRequest); return }
		limit = n
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id,olt_id,COALESCE(pon_id,''),COALESCE(onu_id,''),code,severity,message,value,status,first_seen,last_seen FROM olt_alerts WHERE olt_id=$1 ORDER BY last_seen DESC LIMIT $2`, oltID, limit)
	if err != nil { http.Error(w, "failed to query alerts", http.StatusInternalServerError); return }
	defer rows.Close()
	out := make([]AlertRecord, 0)
	for rows.Next() {
		var x AlertRecord
		if err := rows.Scan(&x.ID,&x.OLTID,&x.PONID,&x.ONUID,&x.Code,&x.Severity,&x.Message,&x.Value,&x.Status,&x.FirstSeen,&x.LastSeen); err != nil { http.Error(w, "failed to read alerts", http.StatusInternalServerError); return }
		out = append(out, x)
	}
	if err := rows.Err(); err != nil { http.Error(w, "failed to read alerts", http.StatusInternalServerError); return }
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
