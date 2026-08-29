package incidents

import (
	"encoding/json"
	"net/http"
	"strings"
)

// API exposes the in-memory incident engine to the NOC UI. Storage can be
// swapped later without changing the HTTP contract.
type API struct { Engine *Engine }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.Engine == nil { http.Error(w, "incident engine is not initialized", 500); return }
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "incidents" { http.NotFound(w,r); return }
	id := parts[1]
	if r.Method == http.MethodGet {
		i, ok := a.Engine.Get(id); if !ok { http.NotFound(w,r); return }; writeJSON(w,i); return
	}
	if r.Method != http.MethodPost || len(parts) != 3 { http.Error(w,"method not allowed",405); return }
	var (i Incident; err error)
	switch parts[2] {
	case "acknowledge": i,err=a.Engine.Acknowledge(id)
	case "resolve": i,err=a.Engine.Resolve(id)
	default: http.NotFound(w,r); return
	}
	if err != nil { http.Error(w,err.Error(),409); return }; writeJSON(w,i)
}

func writeJSON(w http.ResponseWriter, v any) { w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(v) }
