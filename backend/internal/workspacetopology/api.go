package workspacetopology

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

// DTOs mirror the frontend's WorkspaceGroup/CanvasDevice/CanvasLink types
// (frontend/app/(noc)/workspace-topology/types.ts) field-for-field, so the
// store can decode a fetch response with no shape translation. Groups carry
// their bigserial id as a JSON string since WorkspaceGroup.id is typed
// string on the frontend (client-generated ids for local-only groups look
// the same as a stringified backend id).

type GroupDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type DeviceDTO struct {
	ID             string  `json:"id"`
	GroupID        string  `json:"groupId"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	Address        string  `json:"address"`
	SNMPCommunity  string  `json:"snmpCommunity,omitempty"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	LinkedDeviceID *string `json:"linkedDeviceId,omitempty"`
}

type LinkDTO struct {
	ID         string `json:"id"`
	GroupID    string `json:"groupId"`
	SourceID   string `json:"sourceId"`
	TargetID   string `json:"targetId"`
	SourcePort string `json:"sourcePort,omitempty"`
	TargetPort string `json:"targetPort,omitempty"`
	Discovered bool   `json:"discovered"`
}

type FullGroupDTO struct {
	Group   GroupDTO    `json:"group"`
	Devices []DeviceDTO `json:"devices"`
	Links   []LinkDTO   `json:"links"`
}

func groupToDTO(g Group) GroupDTO {
	return GroupDTO{ID: strconv.FormatInt(g.ID, 10), Name: g.Name, CreatedAt: g.CreatedAt.Format(time.RFC3339)}
}

func deviceToDTO(d Device) DeviceDTO {
	return DeviceDTO{
		ID: d.ID, GroupID: strconv.FormatInt(d.GroupID, 10), Name: d.Name, Kind: d.Kind,
		Address: d.Address, SNMPCommunity: d.SNMPCommunity, X: d.PosX, Y: d.PosY, LinkedDeviceID: d.LinkedDeviceID,
	}
}

func linkToDTO(l Link) LinkDTO {
	return LinkDTO{
		ID: l.ID, GroupID: strconv.FormatInt(l.GroupID, 10), SourceID: l.SourceID, TargetID: l.TargetID,
		SourcePort: l.SourcePort, TargetPort: l.TargetPort, Discovered: l.Discovered,
	}
}

// GroupsAPI backs GET/POST /api/v1/workspace-topology/groups.
type GroupsAPI struct{ Repo Repository }

func (a GroupsAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	switch r.Method {
	case http.MethodGet:
		groups, err := a.Repo.ListGroups(ctx, r.URL.Query().Get("tenantId"))
		if err != nil {
			http.Error(w, "failed to load workspace groups", http.StatusInternalServerError)
			return
		}
		out := make([]GroupDTO, 0, len(groups))
		for _, g := range groups {
			out = append(out, groupToDTO(g))
		}
		writeJSON(w, out)

	case http.MethodPost:
		var req struct {
			TenantID string `json:"tenantId"`
			Name     string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		created, err := a.Repo.CreateGroup(ctx, req.TenantID, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, groupToDTO(created))

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GroupAPI backs PUT (rename) / DELETE /api/v1/workspace-topology/groups/{id}
// and GET /api/v1/workspace-topology/groups/{id}/full (group+devices+links).
type GroupAPI struct{ Repo Repository }

func (a GroupAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.RenameGroup(ctx, id, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, groupToDTO(updated))

	case http.MethodDelete:
		if err := a.Repo.DeleteGroup(ctx, id); err != nil {
			http.Error(w, "failed to delete workspace group", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GroupFullAPI backs GET /api/v1/workspace-topology/groups/{id}/full.
type GroupFullAPI struct{ Repo Repository }

func (a GroupFullAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	group, devices, links, err := a.Repo.FullGroup(ctx, id)
	if err != nil {
		http.Error(w, "failed to load workspace group", http.StatusNotFound)
		return
	}
	deviceDTOs := make([]DeviceDTO, 0, len(devices))
	for _, d := range devices {
		deviceDTOs = append(deviceDTOs, deviceToDTO(d))
	}
	linkDTOs := make([]LinkDTO, 0, len(links))
	for _, l := range links {
		linkDTOs = append(linkDTOs, linkToDTO(l))
	}
	writeJSON(w, FullGroupDTO{Group: groupToDTO(group), Devices: deviceDTOs, Links: linkDTOs})
}

// DevicesAPI backs POST /api/v1/workspace-topology/groups/{id}/devices.
type DevicesAPI struct{ Repo Repository }

func (a DevicesAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	groupID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	var dto DeviceDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	created, err := a.Repo.CreateDevice(ctx, Device{
		ID: dto.ID, GroupID: groupID, Name: dto.Name, Kind: dto.Kind, Address: dto.Address,
		SNMPCommunity: dto.SNMPCommunity, PosX: dto.X, PosY: dto.Y, LinkedDeviceID: dto.LinkedDeviceID,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, deviceToDTO(created))
}

// DeviceAPI backs PUT (move/patch) / DELETE /api/v1/workspace-topology/devices/{id}.
type DeviceAPI struct{ Repo Repository }

func (a DeviceAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	id := r.PathValue("id")
	switch r.Method {
	case http.MethodPut:
		var dto DeviceDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		updated, err := a.Repo.UpdateDevice(ctx, id, Device{
			Name: dto.Name, Kind: dto.Kind, Address: dto.Address,
			SNMPCommunity: dto.SNMPCommunity, PosX: dto.X, PosY: dto.Y, LinkedDeviceID: dto.LinkedDeviceID,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, deviceToDTO(updated))

	case http.MethodDelete:
		if err := a.Repo.DeleteDevice(ctx, id); err != nil {
			http.Error(w, "failed to delete device", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// LinksAPI backs POST /api/v1/workspace-topology/groups/{id}/links.
type LinksAPI struct{ Repo Repository }

func (a LinksAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	groupID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid group id", http.StatusBadRequest)
		return
	}
	var dto LinkDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	created, err := a.Repo.CreateLink(ctx, Link{
		ID: dto.ID, GroupID: groupID, SourceID: dto.SourceID, TargetID: dto.TargetID,
		SourcePort: dto.SourcePort, TargetPort: dto.TargetPort, Discovered: dto.Discovered,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, linkToDTO(created))
}

// LinkAPI backs DELETE /api/v1/workspace-topology/links/{id}.
type LinkAPI struct{ Repo Repository }

func (a LinkAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	if err := a.Repo.DeleteLink(ctx, r.PathValue("id")); err != nil {
		http.Error(w, "failed to delete link", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
