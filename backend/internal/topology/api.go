package topology

import (
	"encoding/json"
	"net/http"
)

type API struct { Graph func() Graph }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	if a.Graph == nil { http.Error(w, "topology provider is not initialized", http.StatusServiceUnavailable); return }
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(a.Graph()); err != nil { return }
}
