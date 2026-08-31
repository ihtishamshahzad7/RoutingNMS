package devices

import (
	"encoding/json"
	"net/http"
)

type Handler struct { Repo Repository }

func (h Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w,"method not allowed",405); return }
	var in DeviceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil { http.Error(w,"invalid JSON",400); return }
	if err := ValidateRegistration(in); err != nil { http.Error(w,err.Error(),400); return }
	d, err := h.Repo.Create(r.Context(),in); if err != nil { http.Error(w,"failed to save device",500); return }
	w.Header().Set("Content-Type","application/json"); w.WriteHeader(http.StatusCreated); _=json.NewEncoder(w).Encode(d)
}

func (h Handler) List(w http.ResponseWriter, r *http.Request) {
	org := r.URL.Query().Get("organizationId"); if org == "" { http.Error(w,"organizationId is required",400); return }
	items, err := h.Repo.List(r.Context(),org); if err != nil { http.Error(w,"failed to load devices",500); return }
	w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(items)
}
