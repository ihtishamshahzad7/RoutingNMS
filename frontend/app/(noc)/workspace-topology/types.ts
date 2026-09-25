// Shared types for the Workspace Topology Builder. Kept in one place so the
// store, canvas, discovery service, and monitoring panel all agree on shape
// without importing from each other's implementation files.

export type DeviceKind = "router" | "switch" | "firewall" | "server" | "power";

export type WorkspaceGroup = {
  id: string;
  name: string;
  createdAt: string;
};

export type CanvasDevice = {
  id: string;
  groupId: string;
  name: string;
  kind: DeviceKind;
  address: string;
  snmpCommunity?: string;
  x: number;
  y: number;
  // linked back to a real RoutingNMS device/OLT id when this canvas node
  // represents one, so the monitoring panel can pull real metric_samples
  // instead of mock data once the backend integration lands (see the
  // "not addressed here" note in workspace-topology's README section).
  linkedDeviceId?: string;
};

export type CanvasLink = {
  id: string;
  groupId: string;
  sourceId: string;
  targetId: string;
  sourcePort?: string;
  targetPort?: string;
  discovered: boolean; // true if created by the auto-discovery engine
};

export type MetricPoint = { t: number; value: number };

export type LiveMetrics = {
  bandwidthIn: MetricPoint[];
  bandwidthOut: MetricPoint[];
  latency: MetricPoint[];
  cpu: MetricPoint[];
  memory: MetricPoint[];
};

export type UptimeSegment = {
  start: number;
  end: number;
  status: "up" | "down";
  reason?: string;
};

export type PowerMetrics = {
  voltage: number;
  current: number;
  psu: { id: string; healthy: boolean; label: string }[];
};

// One linked canvas device's real, server-pushed up/latency reading (see
// useLiveStream.ts + backend workspacetopology.Reading). Only fields the
// backend actually has data for are present -- bandwidth/CPU/memory have
// no real collector anywhere in this codebase yet, so they are
// deliberately absent here rather than faked, and stay on the existing
// client-side mock tick.
export type LiveReading = {
  deviceId: string;
  linkedDeviceId: string;
  up?: boolean;
  latencyMs?: number;
};

export type AlertSeverity = "critical" | "warning" | "info";

export type WorkspaceAlert = {
  id: string;
  groupId: string;
  deviceId: string;
  deviceName: string;
  severity: AlertSeverity;
  message: string;
  kind: "link-down" | "unreachable" | "bandwidth" | "cpu" | "memory" | "power";
  firedAt: number;
  acknowledged: boolean;
};
