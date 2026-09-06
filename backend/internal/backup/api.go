package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ExportAPI backs GET /api/v1/backup -- returns the caller's tenant's full
// configuration bundle as a downloadable JSON file.
type ExportAPI struct{ Repos Repositories }

func (a ExportAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	tenantID := r.URL.Query().Get("tenantId")
	bundle, err := Export(ctx, a.Repos, tenantID)
	if err != nil {
		http.Error(w, "failed to build backup: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="routingnms-backup-%s.json"`, time.Now().UTC().Format("2006-01-02")))
	writeJSON(w, bundle)
}

// ImportAPI backs POST /api/v1/backup/import?mode=overwrite|keep|skip --
// accepts a previously exported bundle in the request body and restores it
// into the caller's tenant.
type ImportAPI struct{ Repos Repositories }

func (a ImportAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = ModeSkip
	}
	tenantID := r.URL.Query().Get("tenantId")

	var bundle Bundle
	if err := json.NewDecoder(r.Body).Decode(&bundle); err != nil {
		http.Error(w, "invalid JSON backup file", http.StatusBadRequest)
		return
	}

	result, err := Import(ctx, a.Repos, tenantID, &bundle, mode)
	if err != nil {
		http.Error(w, "import failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, result)
}
