package olt

import (
	"encoding/json"
	"net/http"
	"strings"
)

type Provider interface { GetHierarchy(id string) (Hierarchy, bool) }
type API struct { Provider Provider }

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w,"method not allowed",405); return }
	id:=strings.TrimPrefix(r.URL.Path,"/olts/"); id=strings.Trim(id,"/")
	if id==""||a.Provider==nil { http.Error(w,"OLT not found",404); return }
	h,ok:=a.Provider.GetHierarchy(id); if !ok { http.Error(w,"OLT not found",404); return }
	w.Header().Set("Content-Type","application/json"); _=json.NewEncoder(w).Encode(h)
}
