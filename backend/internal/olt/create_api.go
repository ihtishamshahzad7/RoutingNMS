package olt

import (
	"context"
	"encoding/json"
	"net/http"
)

// CreateHandler backs GET/POST /api/v1/olts: list configured OLTs, and add a
// new one. Adding an OLT also starts polling it immediately (via Runtime),
// so an operator does not need to restart the API for a newly added OLT to
// begin reporting status.
type CreateHandler struct {
	Config  ConfigService
	Runtime *RuntimeManager
}

type createOLTResponse struct {
	OLT     OLT    `json:"olt"`
	Polling bool   `json:"polling"`
	Warning string `json:"warning,omitempty"`
}

func (h CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := h.Config.List(r.Context())
		if err != nil {
			http.Error(w, "failed to load OLTs", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	case http.MethodPost:
		var in CreateInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		created, err := h.Config.Create(r.Context(), in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := createOLTResponse{OLT: created}
		if h.Runtime != nil {
			// Use a background context: starting the poller must outlive
			// this single HTTP request.
			cfg, err := h.Config.LoadOne(context.Background(), created.ID)
			if err != nil {
				// The OLT was still saved successfully — a vendor without a
				// matching profile (see profiles.go: only zte/huawei/
				// fiberhome ship SNMP mappings today) simply can't be
				// polled automatically yet. Report that plainly instead of
				// silently dropping it or failing the whole request.
				resp.Warning = "OLT saved, but automatic polling could not start: " + err.Error()
			} else if err := h.Runtime.StartOne(context.Background(), cfg); err != nil {
				resp.Warning = "OLT saved, but polling failed to start: " + err.Error()
			} else {
				resp.Polling = true
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
