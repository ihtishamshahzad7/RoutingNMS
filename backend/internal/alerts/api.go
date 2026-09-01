package alerts

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// API implements the Sprint 2 HTTP surface: persisted alert-rule CRUD
// (GET/POST /api/v1/alerts/rules), notification-channel CRUD
// (GET/POST /api/v1/alerts/channels), evaluator status
// (GET /api/v1/alerts/evaluator) and the Incident Hub
// (GET/POST /api/v1/alerts/incidents, GET /api/v1/alerts/incidents/{id}).
type API struct {
	Repo      Repository
	Evaluator *Evaluator
}

// ServeHTTP dispatches on method + path suffix, matching the codebase's
// nested-subroute style (see the OLT /alerts handler in main.go).
func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	// path is like api/v1/alerts/rules or api/v1/alerts/incidents/123
	idx := indexOf(parts, "alerts")
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	rest := parts[idx+1:]
	if len(rest) == 0 {
		http.NotFound(w, r)
		return
	}
	switch rest[0] {
	case "rules":
		a.handleRules(w, r, rest[1:])
	case "channels":
		a.handleChannels(w, r, rest[1:])
	case "evaluator":
		a.handleEvaluator(w, r)
	case "incidents":
		a.handleHub(w, r, rest[1:])
	default:
		http.NotFound(w, r)
	}
}

func indexOf(parts []string, want string) int {
	for i, p := range parts {
		if p == want {
			return i
		}
	}
	return -1
}

func (a API) handleRules(w http.ResponseWriter, r *http.Request, rest []string) {
	switch {
	case r.Method == http.MethodGet && len(rest) == 0:
		rules, err := a.Repo.ListRules(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, rules)
	case r.Method == http.MethodPost && len(rest) == 0:
		var rule PersistedRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
			return
		}
		saved, err := a.Repo.SaveRule(r.Context(), rule)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, saved)
	case r.Method == http.MethodPut && len(rest) == 1:
		id, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			http.Error(w, "invalid rule id", http.StatusBadRequest)
			return
		}
		var rule PersistedRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.UpdateRule(r.Context(), id, rule)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, updated)
	case r.Method == http.MethodDelete && len(rest) == 1:
		id, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			http.Error(w, "invalid rule id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.DeleteRule(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (a API) handleChannels(w http.ResponseWriter, r *http.Request, rest []string) {
	switch {
	case r.Method == http.MethodGet && len(rest) == 0:
		channels, err := a.Repo.ListChannels(r.Context(), r.URL.Query().Get("tenantId"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, channels)
	case r.Method == http.MethodPost && len(rest) == 0:
		var ch PersistedChannel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
			return
		}
		saved, err := a.Repo.SaveChannel(r.Context(), ch)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, saved)
	case r.Method == http.MethodDelete && len(rest) == 1:
		id, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			http.Error(w, "invalid channel id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.DeleteChannel(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (a API) handleEvaluator(w http.ResponseWriter, r *http.Request) {
	// GET returns the last-cycle status; POST with ?run=1 triggers an
	// immediate evaluation, then returns the refreshed status.
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.Evaluator == nil {
		http.Error(w, "evaluator is not initialized", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodPost && r.URL.Query().Get("run") == "1" {
		a.Evaluator.EvaluateNow(r.Context())
	}
	writeJSON(w, a.Evaluator.Status())
}

func (a API) handleHub(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items, err := a.Repo.ListAiIncidents(r.Context(),
			r.URL.Query().Get("status"), r.URL.Query().Get("severity"), 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, items)
		return
	}
	id, err := strconv.ParseInt(rest[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid incident id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := a.Repo.GetAiIncident(r.Context(), id)
		if err == sql.ErrNoRows {
			http.Error(w, "incident not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, item)
	case http.MethodPost:
		if len(rest) < 2 {
			http.Error(w, "action required", http.StatusBadRequest)
			return
		}
		status := "open"
		switch rest[1] {
		case "acknowledge":
			status = "acknowledged"
		case "resolve":
			status = "resolved"
		default:
			http.NotFound(w, r)
			return
		}
		if err := a.Repo.SetAiIncidentStatus(r.Context(), id, status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"id": id, "status": status})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}