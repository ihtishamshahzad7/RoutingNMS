package statuspage

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// AdminAPI backs GET/POST /api/v1/status-pages and
// GET/PUT/DELETE /api/v1/status-pages/{id} -- session-authed CRUD for the
// pages themselves (not their public view, see PublicAPI below).
type AdminAPI struct{ Repo Repository }

func (a AdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	idStr := r.PathValue("id")
	switch {
	case r.Method == http.MethodGet && idStr == "":
		pages, err := a.Repo.List(ctx, r.URL.Query().Get("tenantId"))
		if err != nil {
			http.Error(w, "failed to load status pages", http.StatusInternalServerError)
			return
		}
		writeJSON(w, pages)

	case r.Method == http.MethodPost && idStr == "":
		var p Page
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		created, err := a.Repo.Create(ctx, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, created)

	case r.Method == http.MethodGet && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid status page id", http.StatusBadRequest)
			return
		}
		p, err := a.Repo.Get(ctx, id)
		if err != nil {
			http.Error(w, "status page not found", http.StatusNotFound)
			return
		}
		items, err := a.Repo.ListItems(ctx, id)
		if err != nil {
			http.Error(w, "failed to load status page items", http.StatusInternalServerError)
			return
		}
		writeJSON(w, struct {
			Page
			Items []Item `json:"items"`
		}{p, items})

	case r.Method == http.MethodPut && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid status page id", http.StatusBadRequest)
			return
		}
		var p Page
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.Update(ctx, id, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, updated)

	case r.Method == http.MethodDelete && idStr != "":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid status page id", http.StatusBadRequest)
			return
		}
		if err := a.Repo.Delete(ctx, id); err != nil {
			http.Error(w, "failed to delete status page", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ItemsAPI backs PUT /api/v1/status-pages/{id}/items -- session-authed,
// replaces the full ordered item list for a page.
type ItemsAPI struct{ Repo Repository }

func (a ItemsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid status page id", http.StatusBadRequest)
		return
	}
	var req struct {
		Items []Item `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := a.Repo.ReplaceItems(ctx, id, req.Items); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	items, err := a.Repo.ListItems(ctx, id)
	if err != nil {
		http.Error(w, "failed to reload items", http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
}

// PublicAPI backs GET /api/v1/public/status/{slug} -- deliberately
// unauthenticated (this is the whole point of a status page: something you
// can share with a customer who has no RoutingNMS login). A page that is
// unpublished, or a slug that doesn't exist, both 404 -- neither should
// reveal whether the slug was ever valid.
type PublicAPI struct {
	Repo     Repository
	Resolver StatusResolver
}

type publicPageResponse struct {
	Slug                  string       `json:"slug"`
	Title                 string       `json:"title"`
	Description           string       `json:"description"`
	FooterText            string       `json:"footerText"`
	ShowCertificateExpiry bool         `json:"showCertificateExpiry"`
	Items                 []ItemStatus `json:"items"`
	GeneratedAt           time.Time    `json:"generatedAt"`
}

func (a PublicAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	slug := r.PathValue("slug")
	page, err := a.Repo.GetBySlug(ctx, slug)
	if err != nil || !page.Published {
		http.Error(w, "status page not found", http.StatusNotFound)
		return
	}
	items, err := a.Repo.ListItems(ctx, page.ID)
	if err != nil {
		http.Error(w, "failed to load status page", http.StatusInternalServerError)
		return
	}
	statuses, err := a.Resolver.Resolve(ctx, items, page.ShowCertificateExpiry)
	if err != nil {
		http.Error(w, "failed to resolve status", http.StatusInternalServerError)
		return
	}
	writeJSON(w, publicPageResponse{
		Slug: page.Slug, Title: page.Title, Description: page.Description,
		FooterText: page.FooterText, ShowCertificateExpiry: page.ShowCertificateExpiry,
		Items: statuses, GeneratedAt: time.Now().UTC(),
	})
}
