"use client";

import { create } from "zustand";
import type {
  CanvasDevice,
  CanvasLink,
  DeviceKind,
  LiveMetrics,
  WorkspaceAlert,
  WorkspaceGroup,
} from "./types";
import { discoverFromSeed } from "./discovery";
import { generateLiveMetrics, generatePowerMetrics, tickLiveMetrics } from "./mockMetrics";
import { apiFetch, ApiError } from "../../../lib/api";

// Single-tenant placeholder, same convention used by every other (noc) page
// (device-groups, olts, alert-rules, ...) until real multi-tenant auth lands.
const ORG = "tenant-1";

const THRESHOLDS = {
  bandwidthPct: 80, // % of the mocked 1000-unit "capacity" ceiling
  cpuPct: 85,
  memoryPct: 90,
};

type MetricsByDevice = Record<string, LiveMetrics>;

type WorkspaceState = {
  groups: WorkspaceGroup[];
  activeGroupId: string | null;
  devices: CanvasDevice[];
  links: CanvasLink[];
  alerts: WorkspaceAlert[];
  metricsByDevice: MetricsByDevice;
  selectedDeviceId: string | null;
  discovering: boolean;
  loaded: boolean;

  loadGroups: () => Promise<void>;
  createGroup: (name: string) => Promise<void>;
  renameGroup: (id: string, name: string) => void;
  deleteGroup: (id: string) => void;
  setActiveGroup: (id: string | null) => void;

  addDevice: (groupId: string, kind: DeviceKind, x: number, y: number, name?: string) => void;
  moveDevice: (id: string, x: number, y: number) => void;
  updateDevice: (id: string, patch: Partial<CanvasDevice>) => void;
  removeDevice: (id: string) => void;

  addLink: (groupId: string, sourceId: string, targetId: string) => void;
  removeLink: (id: string) => void;

  runDiscovery: (groupId: string, seedDeviceId: string) => Promise<void>;

  selectDevice: (id: string | null) => void;
  tickMetrics: () => void;

  acknowledgeAlert: (id: string) => void;
  clearAcknowledged: () => void;
};

let idCounter = 0;
const nextId = (prefix: string) => `${prefix}-${Date.now()}-${idCounter++}`;

// Persistence is best-effort and additive: every mutation applies to local
// state immediately (so the canvas never waits on a network round trip), and
// fires the matching backend call in the background. A persistence failure
// (backend unreachable, validation error) is swallowed here rather than
// rolled back -- same "local state is the source of truth for this session,
// backend is a durable mirror" tradeoff already made for e.g. tickMetrics.
function persist(fn: () => Promise<unknown>) {
  fn().catch((err) => {
    if (err instanceof ApiError || err instanceof Error) {
      console.warn("workspace-topology: persistence call failed:", err.message);
    }
  });
}

function evaluateAlerts(
  device: CanvasDevice,
  metrics: LiveMetrics,
  existing: WorkspaceAlert[]
): WorkspaceAlert[] {
  const fresh: WorkspaceAlert[] = [];
  const already = (kind: WorkspaceAlert["kind"]) =>
    existing.some((a) => a.deviceId === device.id && a.kind === kind && !a.acknowledged);

  const lastBandwidth = metrics.bandwidthIn[metrics.bandwidthIn.length - 1]?.value ?? 0;
  const lastCpu = metrics.cpu[metrics.cpu.length - 1]?.value ?? 0;
  const lastMemory = metrics.memory[metrics.memory.length - 1]?.value ?? 0;

  if (lastBandwidth > THRESHOLDS.bandwidthPct && !already("bandwidth")) {
    fresh.push({
      id: nextId("alert"),
      groupId: device.groupId,
      deviceId: device.id,
      deviceName: device.name,
      severity: "warning",
      kind: "bandwidth",
      message: `${device.name}: inbound bandwidth at ${lastBandwidth.toFixed(0)}% of capacity`,
      firedAt: Date.now(),
      acknowledged: false,
    });
  }
  if (lastCpu > THRESHOLDS.cpuPct && !already("cpu")) {
    fresh.push({
      id: nextId("alert"),
      groupId: device.groupId,
      deviceId: device.id,
      deviceName: device.name,
      severity: "critical",
      kind: "cpu",
      message: `${device.name}: CPU utilization at ${lastCpu.toFixed(0)}%`,
      firedAt: Date.now(),
      acknowledged: false,
    });
  }
  if (lastMemory > THRESHOLDS.memoryPct && !already("memory")) {
    fresh.push({
      id: nextId("alert"),
      groupId: device.groupId,
      deviceId: device.id,
      deviceName: device.name,
      severity: "warning",
      kind: "memory",
      message: `${device.name}: memory utilization at ${lastMemory.toFixed(0)}%`,
      firedAt: Date.now(),
      acknowledged: false,
    });
  }
  return fresh;
}

export const useWorkspaceStore = create<WorkspaceState>((set, get) => ({
  groups: [],
  activeGroupId: null,
  devices: [],
  links: [],
  alerts: [],
  metricsByDevice: {},
  selectedDeviceId: null,
  discovering: false,
  loaded: false,

  // Loads persisted groups from the backend (call once on mount). Devices
  // and links for a group are loaded lazily by setActiveGroup, since a
  // workspace can have many groups but only one is ever on-screen.
  loadGroups: async () => {
    try {
      const groups = await apiFetch<WorkspaceGroup[]>(`/workspace-topology/groups?tenantId=${ORG}`);
      const currentActive = get().activeGroupId;
      set({ groups, loaded: true });
      if (!currentActive && groups[0]) get().setActiveGroup(groups[0].id);
    } catch (err) {
      // Backend unreachable or not yet migrated -- fall back to a purely
      // local, in-memory workspace rather than blocking the page.
      console.warn("workspace-topology: failed to load groups, continuing locally:", err);
      set({ loaded: true });
    }
  },

  createGroup: async (name) => {
    try {
      const group = await apiFetch<WorkspaceGroup>(`/workspace-topology/groups`, {
        method: "POST",
        body: JSON.stringify({ tenantId: ORG, name }),
      });
      set((s) => ({ groups: [...s.groups, group], activeGroupId: s.activeGroupId ?? group.id }));
    } catch (err) {
      // Persistence unavailable -- still let the user work with a
      // local-only group for this session rather than blocking them.
      console.warn("workspace-topology: failed to persist new group, using local-only group:", err);
      set((s) => {
        const group: WorkspaceGroup = { id: nextId("group"), name, createdAt: new Date().toISOString() };
        return { groups: [...s.groups, group], activeGroupId: s.activeGroupId ?? group.id };
      });
    }
  },

  renameGroup: (id, name) => {
    set((s) => ({ groups: s.groups.map((g) => (g.id === id ? { ...g, name } : g)) }));
    persist(() => apiFetch(`/workspace-topology/groups/${id}`, { method: "PUT", body: JSON.stringify({ name }) }));
  },

  deleteGroup: (id) => {
    set((s) => ({
      groups: s.groups.filter((g) => g.id !== id),
      devices: s.devices.filter((d) => d.groupId !== id),
      links: s.links.filter((l) => l.groupId !== id),
      alerts: s.alerts.filter((a) => a.groupId !== id),
      activeGroupId: s.activeGroupId === id ? null : s.activeGroupId,
    }));
    persist(() => apiFetch(`/workspace-topology/groups/${id}`, { method: "DELETE" }));
  },

  setActiveGroup: (id) => {
    set({ activeGroupId: id, selectedDeviceId: null });
    if (!id) return;
    // Load this group's persisted devices/links (skip re-fetching ones
    // already in memory from a prior visit this session).
    apiFetch<{ devices: CanvasDevice[]; links: CanvasLink[] }>(`/workspace-topology/groups/${id}/full`)
      .then(({ devices, links }) => {
        set((s) => ({
          devices: [...s.devices.filter((d) => d.groupId !== id), ...devices],
          links: [...s.links.filter((l) => l.groupId !== id), ...links],
          metricsByDevice: devices.reduce(
            (acc, d) => ({ ...acc, [d.id]: acc[d.id] ?? generateLiveMetrics(d.name) }),
            { ...get().metricsByDevice }
          ),
        }));
      })
      .catch((err) => {
        // Not persisted yet (e.g. a local-only group created before the
        // backend was reachable) -- keep whatever's already in memory.
        console.warn("workspace-topology: failed to load group contents:", err);
      });
  },

  addDevice: (groupId, kind, x, y, name) => {
    const device: CanvasDevice = {
      id: nextId("dev"),
      groupId,
      name: name || `${kind}-${Math.floor(Math.random() * 900 + 100)}`,
      kind,
      address: "",
      x,
      y,
    };
    set((s) => ({
      devices: [...s.devices, device],
      metricsByDevice: { ...s.metricsByDevice, [device.id]: generateLiveMetrics(device.name) },
    }));
    persist(() => apiFetch(`/workspace-topology/groups/${groupId}/devices`, { method: "POST", body: JSON.stringify(device) }));
  },

  moveDevice: (id, x, y) => {
    set((s) => ({ devices: s.devices.map((d) => (d.id === id ? { ...d, x, y } : d)) }));
    const device = get().devices.find((d) => d.id === id);
    if (device) persist(() => apiFetch(`/workspace-topology/devices/${id}`, { method: "PUT", body: JSON.stringify(device) }));
  },

  updateDevice: (id, patch) => {
    set((s) => ({ devices: s.devices.map((d) => (d.id === id ? { ...d, ...patch } : d)) }));
    const device = get().devices.find((d) => d.id === id);
    if (device) persist(() => apiFetch(`/workspace-topology/devices/${id}`, { method: "PUT", body: JSON.stringify(device) }));
  },

  removeDevice: (id) => {
    set((s) => ({
      devices: s.devices.filter((d) => d.id !== id),
      links: s.links.filter((l) => l.sourceId !== id && l.targetId !== id),
      selectedDeviceId: s.selectedDeviceId === id ? null : s.selectedDeviceId,
    }));
    persist(() => apiFetch(`/workspace-topology/devices/${id}`, { method: "DELETE" }));
  },

  addLink: (groupId, sourceId, targetId) => {
    if (sourceId === targetId) return;
    const exists = get().links.some(
      (l) =>
        (l.sourceId === sourceId && l.targetId === targetId) ||
        (l.sourceId === targetId && l.targetId === sourceId)
    );
    if (exists) return;
    const link: CanvasLink = { id: nextId("link"), groupId, sourceId, targetId, discovered: false };
    set((s) => ({ links: [...s.links, link] }));
    persist(() => apiFetch(`/workspace-topology/groups/${groupId}/links`, { method: "POST", body: JSON.stringify(link) }));
  },

  removeLink: (id) => {
    set((s) => ({ links: s.links.filter((l) => l.id !== id) }));
    persist(() => apiFetch(`/workspace-topology/links/${id}`, { method: "DELETE" }));
  },

  runDiscovery: async (groupId, seedDeviceId) => {
    const seed = get().devices.find((d) => d.id === seedDeviceId);
    if (!seed) return;
    set({ discovering: true });
    try {
      const result = await discoverFromSeed(groupId, seed);
      set((s) => {
        const newMetrics = { ...s.metricsByDevice };
        for (const d of result.devices) newMetrics[d.id] = generateLiveMetrics(d.name);
        return {
          devices: [...s.devices, ...result.devices],
          links: [...s.links, ...result.links],
          metricsByDevice: newMetrics,
        };
      });
      for (const d of result.devices) {
        persist(() => apiFetch(`/workspace-topology/groups/${groupId}/devices`, { method: "POST", body: JSON.stringify(d) }));
      }
      for (const l of result.links) {
        persist(() => apiFetch(`/workspace-topology/groups/${groupId}/links`, { method: "POST", body: JSON.stringify(l) }));
      }
    } finally {
      set({ discovering: false });
    }
  },

  selectDevice: (id) => set({ selectedDeviceId: id }),

  tickMetrics: () =>
    set((s) => {
      const nextMetrics: MetricsByDevice = {};
      let newAlerts: WorkspaceAlert[] = [];
      for (const device of s.devices) {
        const current = s.metricsByDevice[device.id] ?? generateLiveMetrics(device.name);
        const updated = tickLiveMetrics(current);
        nextMetrics[device.id] = updated;
        newAlerts = newAlerts.concat(evaluateAlerts(device, updated, s.alerts));
      }
      return {
        metricsByDevice: nextMetrics,
        alerts: newAlerts.length ? [...newAlerts, ...s.alerts].slice(0, 200) : s.alerts,
      };
    }),

  acknowledgeAlert: (id) =>
    set((s) => ({ alerts: s.alerts.map((a) => (a.id === id ? { ...a, acknowledged: true } : a)) })),

  clearAcknowledged: () => set((s) => ({ alerts: s.alerts.filter((a) => !a.acknowledged) })),
}));

export { generatePowerMetrics };
