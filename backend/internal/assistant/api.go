package assistant

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// API backs POST /api/v1/ai/assistant -- the NOC AI chat widget (Screen 5).
// Session-authed, following the provisioning/templates idiom; answers are the
// deterministic, backend-grounded replies from Repository.Answer.
type API struct{ Repo Repository }

type questionReq struct {
	Message string `json:"message"`
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in questionReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if in.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	ans, err := a.Repo.Answer(ctx, in.Message)
	if err != nil {
		http.Error(w, "failed to answer: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, ans)
}