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

  createGroup: (name: string) => void;
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

  createGroup: (name) =>
    set((s) => {
      const group: WorkspaceGroup = { id: nextId("group"), name, createdAt: new Date().toISOString() };
      return { groups: [...s.groups, group], activeGroupId: s.activeGroupId ?? group.id };
    }),

  renameGroup: (id, name) =>
    set((s) => ({ groups: s.groups.map((g) => (g.id === id ? { ...g, name } : g)) })),

  deleteGroup: (id) =>
    set((s) => ({
      groups: s.groups.filter((g) => g.id !== id),
      devices: s.devices.filter((d) => d.groupId !== id),
      links: s.links.filter((l) => l.groupId !== id),
      alerts: s.alerts.filter((a) => a.groupId !== id),
      activeGroupId: s.activeGroupId === id ? null : s.activeGroupId,
    })),

  setActiveGroup: (id) => set({ activeGroupId: id, selectedDeviceId: null }),

  addDevice: (groupId, kind, x, y, name) =>
    set((s) => {
      const device: CanvasDevice = {
        id: nextId("dev"),
        groupId,
        name: name || `${kind}-${Math.floor(Math.random() * 900 + 100)}`,
        kind,
        address: "",
        x,
        y,
      };
      return {
        devices: [...s.devices, device],
        metricsByDevice: { ...s.metricsByDevice, [device.id]: generateLiveMetrics(device.name) },
      };
    }),

  moveDevice: (id, x, y) =>
    set((s) => ({ devices: s.devices.map((d) => (d.id === id ? { ...d, x, y } : d)) })),

  updateDevice: (id, patch) =>
    set((s) => ({ devices: s.devices.map((d) => (d.id === id ? { ...d, ...patch } : d)) })),

  removeDevice: (id) =>
    set((s) => ({
      devices: s.devices.filter((d) => d.id !== id),
      links: s.links.filter((l) => l.sourceId !== id && l.targetId !== id),
      selectedDeviceId: s.selectedDeviceId === id ? null : s.selectedDeviceId,
    })),

  addLink: (groupId, sourceId, targetId) =>
    set((s) => {
      if (sourceId === targetId) return {};
      const exists = s.links.some(
        (l) =>
          (l.sourceId === sourceId && l.targetId === targetId) ||
          (l.sourceId === targetId && l.targetId === sourceId)
      );
      if (exists) return {};
      const link: CanvasLink = { id: nextId("link"), groupId, sourceId, targetId, discovered: false };
      return { links: [...s.links, link] };
    }),

  removeLink: (id) => set((s) => ({ links: s.links.filter((l) => l.id !== id) })),

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
