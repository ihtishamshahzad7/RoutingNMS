// Mocked real-time metrics + power telemetry for the monitoring panel and
// alerting engine. Real integration point (not built this sprint): the
// existing Postgres metric_samples pipeline already records per-device
// bandwidth/latency/cpu/memory-shaped series for RoutingNMS devices, and
// power/PSU telemetry would extend the same OLT/PON/ONU-style sampler --
// see backend/internal/metricsdb. Swapping this generator for a real
// `GET /api/v1/devices/{id}/live-metrics` poll (or a WebSocket push) is a
// drop-in replacement as long as the caller keeps using LiveMetrics/
// PowerMetrics/UptimeSegment from ./types.

import type { LiveMetrics, MetricPoint, PowerMetrics, UptimeSegment } from "./types";

function series(base: number, jitter: number, points: number, now: number, stepMs: number): MetricPoint[] {
  const out: MetricPoint[] = [];
  let v = base;
  for (let i = points - 1; i >= 0; i--) {
    v = Math.max(0, v + (Math.random() - 0.5) * jitter);
    out.push({ t: now - i * stepMs, value: Math.round(v * 100) / 100 });
  }
  return out;
}

export function generateLiveMetrics(seedName: string): LiveMetrics {
  const now = Date.now();
  const stepMs = 5000;
  const points = 30;
  // Seed a stable-looking baseline per device name so a panel reopened for
  // the same device doesn't look wildly different each time.
  let seed = 0;
  for (let i = 0; i < seedName.length; i++) seed += seedName.charCodeAt(i);
  const rnd = () => (Math.sin(seed++) + 1) / 2;

  return {
    bandwidthIn: series(20 + rnd() * 60, 8, points, now, stepMs),
    bandwidthOut: series(15 + rnd() * 40, 6, points, now, stepMs),
    latency: series(2 + rnd() * 15, 3, points, now, stepMs),
    cpu: series(20 + rnd() * 40, 6, points, now, stepMs),
    memory: series(30 + rnd() * 35, 4, points, now, stepMs),
  };
}

export function tickLiveMetrics(prev: LiveMetrics): LiveMetrics {
  const now = Date.now();
  const advance = (pts: MetricPoint[], jitter: number, floor = 0, ceil = 100): MetricPoint[] => {
    const last = pts[pts.length - 1]?.value ?? 0;
    const next = Math.min(ceil, Math.max(floor, last + (Math.random() - 0.5) * jitter));
    return [...pts.slice(1), { t: now, value: Math.round(next * 100) / 100 }];
  };
  return {
    bandwidthIn: advance(prev.bandwidthIn, 8, 0, 1000),
    bandwidthOut: advance(prev.bandwidthOut, 6, 0, 1000),
    latency: advance(prev.latency, 3, 0, 250),
    cpu: advance(prev.cpu, 6, 0, 100),
    memory: advance(prev.memory, 4, 0, 100),
  };
}

export function generatePowerMetrics(healthy: boolean): PowerMetrics {
  return {
    voltage: healthy ? 11.9 + Math.random() * 0.3 : 9.5 + Math.random() * 0.8,
    current: 2 + Math.random() * 1.5,
    psu: [
      { id: "psu-1", label: "PSU 1 (primary)", healthy: true },
      { id: "psu-2", label: "PSU 2 (redundant)", healthy },
    ],
  };
}

export function generateUptimeTimeline(): UptimeSegment[] {
  const now = Date.now();
  const day = 24 * 60 * 60 * 1000;
  const segments: UptimeSegment[] = [];
  let cursor = now - 7 * day;
  while (cursor < now) {
    const down = Math.random() < 0.12;
    const duration = down ? (5 + Math.random() * 40) * 60 * 1000 : (2 + Math.random() * 20) * 60 * 60 * 1000;
    segments.push({
      start: cursor,
      end: Math.min(now, cursor + duration),
      status: down ? "down" : "up",
      reason: down ? ["ICMP timeout", "SNMP unreachable", "Link flap"][Math.floor(Math.random() * 3)] : undefined,
    });
    cursor += duration;
  }
  return segments;
}
