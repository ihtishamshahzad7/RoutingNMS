package topolinks

import (
	"encoding/json"
	"net/http"
	"strings"
)

// GroupsAPI backs GET/POST /api/v1/topology-groups.
type GroupsAPI struct {
	Repo Repository
}

func (a GroupsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		org := r.URL.Query().Get("organizationId")
		if org == "" {
			http.Error(w, "organizationId is required", 400)
			return
		}
		groups, err := a.Repo.ListGroups(r.Context(), org)
		if err != nil {
			http.Error(w, "failed to load topology groups: "+err.Error(), 500)
			return
		}
		writeJSON(w, groups)
	case http.MethodPost:
		var in struct {
			OrganizationID string `json:"organizationId"`
			Name           string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" || in.OrganizationID == "" {
			http.Error(w, "organizationId and name are required", 400)
			return
		}
		g, err := a.Repo.CreateGroup(r.Context(), in.OrganizationID, in.Name)
		if err != nil {
			http.Error(w, "failed to create topology group: "+err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, g)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// GroupAPI backs DELETE /api/v1/topology-groups/{id}.
type GroupAPI struct{ Repo Repository }

func (a GroupAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := r.PathValue("id")
	if err := a.Repo.DeleteGroup(r.Context(), id); err != nil {
		http.Error(w, "failed to delete topology group: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LinksAPI backs GET/POST /api/v1/topology-groups/{id}/links.
type LinksAPI struct{ Repo Repository }

func (a LinksAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	groupID := r.PathValue("id")
	if groupID == "" {
		http.Error(w, "group ID is required", 400)
		return
	}
	switch r.Method {
	case http.MethodGet:
		links, err := a.Repo.ListLinks(r.Context(), groupID)
		if err != nil {
			http.Error(w, "failed to load topology links: "+err.Error(), 500)
			return
		}
		writeJSON(w, links)
	case http.MethodPost:
		var in struct {
			DeviceAID  string `json:"deviceAId"`
			InterfaceA string `json:"interfaceA"`
			DeviceBID  string `json:"deviceBId"`
			InterfaceB string `json:"interfaceB"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		if in.DeviceAID == "" || in.DeviceBID == "" || strings.TrimSpace(in.InterfaceA) == "" || strings.TrimSpace(in.InterfaceB) == "" {
			http.Error(w, "deviceAId, interfaceA, deviceBId and interfaceB are all required", 400)
			return
		}
		link, err := a.Repo.CreateLink(r.Context(), groupID, in.DeviceAID, in.InterfaceA, in.DeviceBID, in.InterfaceB)
		if err != nil {
			http.Error(w, "failed to create topology link: "+err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, link)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// LinkAPI backs DELETE /api/v1/topology-links/{id}.
type LinkAPI struct{ Repo Repository }

func (a LinkAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", 405)
		return
	}
	id := r.PathValue("id")
	if err := a.Repo.DeleteLink(r.Context(), id); err != nil {
		http.Error(w, "failed to delete topology link: "+err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// StatusAPI backs GET /api/v1/topology-groups/{id}/status -- live up/down
// for every link in a group, polled periodically by the frontend editor.
type StatusAPI struct {
	Repo   Repository
	Poller *Poller
}

func (a StatusAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	groupID := r.PathValue("id")
	links, err := a.Repo.ListLinks(r.Context(), groupID)
	if err != nil {
		http.Error(w, "failed to load topology links: "+err.Error(), 500)
		return
	}
	ids := make([]string, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.ID)
	}
	var statuses map[string]LinkStatus
	if a.Poller != nil {
		statuses = a.Poller.LiveForGroup(ids)
	} else {
		statuses = map[string]LinkStatus{}
	}
	writeJSON(w, statuses)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
