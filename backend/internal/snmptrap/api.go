package snmptrap

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// RulesAPI backs GET/POST /api/v1/traps/rules and DELETE /api/v1/traps/rules/{id}.
type RulesAPI struct{ Repo Repository }

func (a RulesAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		rules, err := a.Repo.ListRules(ctx)
		if err != nil {
			http.Error(w, "failed to load trap rules", http.StatusInternalServerError)
			return
		}
		writeJSON(w, rules)

	case http.MethodPost:
		var in struct {
			Name             string `json:"name"`
			OIDPattern       string `json:"oidPattern"`
			Severity         string `json:"severity"`
			Enabled          *bool  `json:"enabled"`
			NotifyEmail      string `json:"notifyEmail"`
			NotifyWebhookURL string `json:"notifyWebhookUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if in.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		enabled := true
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		rule := Rule{
			Name:             in.Name,
			OIDPattern:       in.OIDPattern,
			Severity:         in.Severity,
			Enabled:          enabled,
			NotifyEmail:      in.NotifyEmail,
			NotifyWebhookURL: in.NotifyWebhookURL,
		}
		created, err := a.Repo.CreateRule(ctx, rule)
		if err != nil {
			http.Error(w, "failed to create trap rule", http.StatusInternalServerError)
			return
		}
		writeJSON(w, created)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// RuleAPI backs DELETE /api/v1/traps/rules/{id} (a separate handler because
// this project's ServeMux path patterns key off the trailing segment).
type RuleAPI struct{ Repo Repository }

func (a RuleAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid rule id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := a.Repo.DeleteRule(ctx, id); err != nil {
		http.Error(w, "failed to delete trap rule", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TrapsAPI backs GET /api/v1/traps (recent received traps).
type TrapsAPI struct{ Repo Repository }

func (a TrapsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	limit := 200
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 1000 {
		limit = v
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	traps, err := a.Repo.List(ctx, limit, q.Get("sourceIp"))
	if err != nil {
		http.Error(w, "failed to load traps", http.StatusInternalServerError)
		return
	}
	writeJSON(w, traps)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
