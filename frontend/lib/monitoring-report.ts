// Download Report PDF (6-page rebuild, item 2.4): fetches raw ICMP and
// Port/Service history plus maintenance-window coverage for an arbitrary
// past date range, computes uptime %/avg latency/up-down events per check
// type (excluding paused/maintenance spans from the uptime % calculation,
// per the user's explicit instruction), and renders a PDF client-side with
// jsPDF + jspdf-autotable. No backend/Go dependency: PDF generation lives
// entirely in the frontend so the new npm packages are the only new
// dependency risk (there's no local Go toolchain to verify a new Go
// dependency against).
//
// Item 3.6 polish pass: extended (still frontend-only) to also pull, for
// SNMP-enabled devices, per-port up/down history (GET
// /api/v1/interfaces/{id}/history-range, extended in this same item to
// accept ?from=&to= like ping/portcheck already did) and CPU/memory
// threshold-breach events (GET /api/v1/events?device=...&eventType=
// high_cpu|high_memory, item 3.5's unified events). No maintenance-window
// exclusion is applied to this SNMP data -- maintenance windows in this
// app model planned device-connectivity downtime, not port link flaps or
// CPU/memory alerts, so there's nothing to exclude.

import { apiFetch, ApiError } from "./api";

export type RawSample = { t: number; reachable: boolean; latencyMs: number | null };

export type MaintenanceInterval = { start: number; end: number; title: string };

export type CheckReport = {
  label: string; // "ICMP" | "Port/Service"
  configured: boolean;
  uptimePct: number | null; // excludes maintenance-covered time from both numerator and denominator
  avgLatencyMs: number | null;
  events: DownEvent[];
  maintenanceMs: number; // total time excluded as maintenance within the range
};

export type DownEvent = {
  wentDownAt: number;
  cameBackUpAt: number | null; // null = still down at range end
  durationMs: number;
};

// Item 3.6: one interface's down period within the report range, computed
// the same way computeEvents() detects ICMP/Port-check down periods --
// just on operUp instead of reachable, and with no maintenance exclusion
// (see file header).
export type PortDownEvent = {
  portName: string;
  wentDownAt: number;
  cameBackUpAt: number | null;
  durationMs: number;
};

// Item 3.6: one CPU/memory threshold crossing (events.EventHighCPU /
// EventHighMemory) within the report range.
export type ThresholdEvent = {
  label: string; // "High CPU" | "High memory"
  message: string;
  occurredAt: number;
  count: number;
};

export type ReportData = {
  deviceName: string;
  deviceAddress: string;
  rangeFrom: number;
  rangeTo: number;
  paused: boolean; // device.enabled === false *right now* -- see caveat in the PDF footer
  icmp: CheckReport;
  port: CheckReport;
  // Item 3.6: present only when the device is SNMP-enabled; both arrays
  // are empty (not omitted) when SNMP is enabled but nothing happened in
  // range, so the PDF can print "none in this range" instead of skipping
  // the section entirely.
  snmpEnabled: boolean;
  portEvents: PortDownEvent[];
  hostMetricEvents: ThresholdEvent[];
};

type Device = {
  id: string; name: string; address: string; enabled: boolean;
  icmpEnabled: boolean;
  portCheckEnabled: boolean; portCheckProtocol: string; portCheckPort: number;
  snmpEnabled: boolean;
};
type ProbeResult = { probedAt: string; rttMs?: number | null; isReachable: boolean };
type PortHistoryPoint = { probedAt: string; latencyMs?: number | null; isReachable: boolean };
type InterfaceLite = { id: number; name: string; ifIndex: number };
type PortMetricPoint = { probedAt: string; operUp: boolean };
type EventRecord = { eventType: string; message: string; count: number; firstSeen: string; lastSeen: string };

const HOST_METRIC_EVENT_LABEL: Record<string, string> = { high_cpu: "High CPU", high_memory: "High memory" };

/** Down/up transitions in a port's operUp series, mirroring computeEvents()
 * above but on a boolean field with no maintenance-window filtering (see
 * file header) and no latency to average. */
function computePortEvents(portName: string, points: PortMetricPoint[], rangeTo: number): PortDownEvent[] {
  const events: PortDownEvent[] = [];
  let downSince: number | null = null;
  for (const p of points) {
    const t = new Date(p.probedAt).getTime();
    if (!p.operUp && downSince == null) {
      downSince = t;
    } else if (p.operUp && downSince != null) {
      events.push({ portName, wentDownAt: downSince, cameBackUpAt: t, durationMs: t - downSince });
      downSince = null;
    }
  }
  if (downSince != null) {
    events.push({ portName, wentDownAt: downSince, cameBackUpAt: null, durationMs: rangeTo - downSince });
  }
  return events;
}

const ORG = "tenant-1";

function toRFC3339(ms: number): string {
  return new Date(ms).toISOString();
}

/** Merges overlapping/adjacent maintenance intervals so duration math below
 * never double-counts a moment covered by two windows at once. */
function mergeIntervals(intervals: MaintenanceInterval[]): MaintenanceInterval[] {
  if (intervals.length === 0) return [];
  const sorted = [...intervals].sort((a, b) => a.start - b.start);
  const out: MaintenanceInterval[] = [{ ...sorted[0] }];
  for (const cur of sorted.slice(1)) {
    const last = out[out.length - 1];
    if (cur.start <= last.end) {
      last.end = Math.max(last.end, cur.end);
    } else {
      out.push({ ...cur });
    }
  }
  return out;
}

function isUnderMaintenance(t: number, intervals: MaintenanceInterval[]): boolean {
  return intervals.some((iv) => t >= iv.start && t < iv.end);
}

function totalMaintenanceMs(intervals: MaintenanceInterval[]): number {
  return intervals.reduce((sum, iv) => sum + (iv.end - iv.start), 0);
}

/** Detects up/down transitions in a raw sample series and computes each
 * down period's duration, dropping samples that fall inside a maintenance
 * interval before transition detection so planned downtime never shows up
 * as a real outage event. */
function computeEvents(samples: RawSample[], intervals: MaintenanceInterval[], rangeTo: number): DownEvent[] {
  const live = samples.filter((s) => !isUnderMaintenance(s.t, intervals));
  const events: DownEvent[] = [];
  let downSince: number | null = null;
  for (const s of live) {
    if (!s.reachable && downSince == null) {
      downSince = s.t;
    } else if (s.reachable && downSince != null) {
      events.push({ wentDownAt: downSince, cameBackUpAt: s.t, durationMs: s.t - downSince });
      downSince = null;
    }
  }
  if (downSince != null) {
    events.push({ wentDownAt: downSince, cameBackUpAt: null, durationMs: rangeTo - downSince });
  }
  return events;
}

function computeCheckReport(label: string, configured: boolean, samples: RawSample[], intervals: MaintenanceInterval[], rangeFrom: number, rangeTo: number): CheckReport {
  const maintenanceMs = totalMaintenanceMs(mergeIntervals(intervals));
  if (!configured) {
    return { label, configured: false, uptimePct: null, avgLatencyMs: null, events: [], maintenanceMs };
  }
  const live = samples.filter((s) => !isUnderMaintenance(s.t, intervals));
  const uptimePct = live.length ? (live.filter((s) => s.reachable).length / live.length) * 100 : null;
  const latencies = live.map((s) => s.latencyMs).filter((v): v is number => v != null);
  const avgLatencyMs = latencies.length ? latencies.reduce((a, b) => a + b, 0) / latencies.length : null;
  const events = computeEvents(samples, mergeIntervals(intervals), rangeTo);
  return { label, configured: true, uptimePct, avgLatencyMs, events, maintenanceMs };
}

export async function buildReportData(deviceId: string, from: Date, to: Date): Promise<ReportData> {
  const rangeFrom = from.getTime();
  const rangeTo = to.getTime();
  const fromQ = toRFC3339(rangeFrom);
  const toQ = toRFC3339(rangeTo);

  const devices = await apiFetch<Device[]>(`/devices?organizationId=${ORG}`);
  const device = devices.find((d) => d.id === deviceId);
  if (!device) throw new Error("Device not found.");

  const [icmpHistory, portHistory, intervalsRes] = await Promise.all([
    device.icmpEnabled
      ? apiFetch<{ history: ProbeResult[] }>(`/ping/${deviceId}/history-range?from=${fromQ}&to=${toQ}`).then((r) => r.history)
      : Promise.resolve<ProbeResult[]>([]),
    device.portCheckEnabled
      ? apiFetch<{ history: PortHistoryPoint[] }>(`/port-check/${deviceId}/history-range?from=${fromQ}&to=${toQ}`).then((r) => r.history)
      : Promise.resolve<PortHistoryPoint[]>([]),
    apiFetch<{ intervals: { start: string; end: string; title: string }[] }>(
      `/maintenance-windows/for-device/${deviceId}?from=${fromQ}&to=${toQ}`
    ).catch((e) => {
      // Non-fatal: an older/unavailable maintenance module just means the
      // report proceeds with no maintenance exclusions rather than failing.
      if (e instanceof ApiError) return { intervals: [] };
      return { intervals: [] };
    }),
  ]);

  const intervals: MaintenanceInterval[] = intervalsRes.intervals.map((iv) => ({
    start: new Date(iv.start).getTime(),
    end: new Date(iv.end).getTime(),
    title: iv.title,
  }));

  const icmpSamples: RawSample[] = icmpHistory.map((p) => ({
    t: new Date(p.probedAt).getTime(),
    reachable: p.isReachable,
    latencyMs: p.rttMs ?? null,
  }));
  const portSamples: RawSample[] = portHistory.map((p) => ({
    t: new Date(p.probedAt).getTime(),
    reachable: p.isReachable,
    latencyMs: p.latencyMs ?? null,
  }));

  // Item 3.6: SNMP port up/down history + CPU/memory threshold events,
  // only for SNMP-enabled devices -- fetched after the device lookup above
  // since it needs device.snmpEnabled to decide whether to bother.
  let portEvents: PortDownEvent[] = [];
  let hostMetricEvents: ThresholdEvent[] = [];
  if (device.snmpEnabled) {
    try {
      const interfaces = await apiFetch<InterfaceLite[]>(`/devices/${deviceId}/interfaces`);
      const perPort = await Promise.all(
        interfaces.map((iface) =>
          apiFetch<{ history: PortMetricPoint[] }>(`/interfaces/${iface.id}/history-range?from=${fromQ}&to=${toQ}`)
            .then((r) => computePortEvents(iface.name || `if${iface.ifIndex}`, r.history, rangeTo))
            .catch(() => [] as PortDownEvent[])
        )
      );
      portEvents = perPort.flat().sort((a, b) => a.wentDownAt - b.wentDownAt);
    } catch {
      // Non-fatal: the report still generates without the SNMP sections.
    }
    try {
      const [cpuRes, memRes] = await Promise.all([
        apiFetch<{ items: EventRecord[] }>(`/events?device=${deviceId}&eventType=high_cpu&from=${fromQ}&to=${toQ}&limit=1000`),
        apiFetch<{ items: EventRecord[] }>(`/events?device=${deviceId}&eventType=high_memory&from=${fromQ}&to=${toQ}&limit=1000`),
      ]);
      hostMetricEvents = [...cpuRes.items, ...memRes.items]
        .map((ev) => ({
          label: HOST_METRIC_EVENT_LABEL[ev.eventType] ?? ev.eventType,
          message: ev.message,
          occurredAt: new Date(ev.firstSeen).getTime(),
          count: ev.count,
        }))
        .sort((a, b) => a.occurredAt - b.occurredAt);
    } catch {
      // Non-fatal, same as above.
    }
  }

  return {
    deviceName: device.name,
    deviceAddress: device.address,
    rangeFrom,
    rangeTo,
    paused: !device.enabled,
    icmp: computeCheckReport("ICMP", device.icmpEnabled, icmpSamples, intervals, rangeFrom, rangeTo),
    port: computeCheckReport(
      `Port/Service (${device.portCheckProtocol.toUpperCase()}${device.portCheckProtocol === "tcp" ? "" : ""}${device.portCheckPort ? " :" + device.portCheckPort : ""})`,
      device.portCheckEnabled,
      portSamples,
      intervals,
      rangeFrom,
      rangeTo
    ),
    snmpEnabled: device.snmpEnabled,
    portEvents,
    hostMetricEvents,
  };
}

function fmtDuration(ms: number): string {
  const totalMin = Math.round(ms / 60000);
  const days = Math.floor(totalMin / 1440);
  const hours = Math.floor((totalMin % 1440) / 60);
  const mins = totalMin % 60;
  const parts: string[] = [];
  if (days) parts.push(`${days}d`);
  if (hours) parts.push(`${hours}h`);
  if (mins || parts.length === 0) parts.push(`${mins}m`);
  return parts.join(" ");
}

export async function generateReportPDF(data: ReportData): Promise<void> {
  const { jsPDF } = await import("jspdf");
  await import("jspdf-autotable");
  const doc = new jsPDF();

  const fromStr = new Date(data.rangeFrom).toLocaleString();
  const toStr = new Date(data.rangeTo).toLocaleString();

  doc.setFontSize(16);
  doc.text(`Connectivity Report — ${data.deviceName}`, 14, 18);
  doc.setFontSize(10);
  doc.setTextColor(90, 100, 110);
  doc.text(`${data.deviceAddress}`, 14, 25);
  doc.text(`Range: ${fromStr} — ${toStr}`, 14, 31);
  if (data.paused) {
    doc.setTextColor(199, 119, 0);
    doc.text(`Note: monitoring is currently paused for this device.`, 14, 37);
  }
  doc.setTextColor(31, 42, 55);

  const checks = [data.icmp, data.port].filter((c) => c.configured);

  let y = data.paused ? 45 : 40;
  (doc as any).autoTable({
    startY: y,
    head: [["Check", "Uptime %", "Avg latency/response", "Maintenance excluded", "Down events"]],
    body: checks.map((c) => [
      c.label,
      c.uptimePct != null ? `${c.uptimePct.toFixed(2)}%` : "no data",
      c.avgLatencyMs != null ? `${c.avgLatencyMs.toFixed(1)} ms` : "no data",
      c.maintenanceMs > 0 ? fmtDuration(c.maintenanceMs) : "none",
      String(c.events.length),
    ]),
    styles: { fontSize: 9 },
    headStyles: { fillColor: [46, 123, 246] },
  });
  y = (doc as any).lastAutoTable.finalY + 8;

  for (const c of checks) {
    if (y > 260) {
      doc.addPage();
      y = 18;
    }
    doc.setFontSize(12);
    doc.text(`${c.label} — up/down events`, 14, y);
    y += 4;
    if (c.events.length === 0) {
      doc.setFontSize(9);
      doc.setTextColor(90, 100, 110);
      doc.text("No down events in this range.", 14, y + 6);
      doc.setTextColor(31, 42, 55);
      y += 14;
      continue;
    }
    (doc as any).autoTable({
      startY: y + 2,
      head: [["Went down", "Came back up", "Duration"]],
      body: c.events.map((e) => [
        new Date(e.wentDownAt).toLocaleString(),
        e.cameBackUpAt != null ? new Date(e.cameBackUpAt).toLocaleString() : "still down at end of range",
        fmtDuration(e.durationMs),
      ]),
      styles: { fontSize: 8 },
      headStyles: { fillColor: [92, 107, 122] },
    });
    y = (doc as any).lastAutoTable.finalY + 8;
  }

  // Item 3.6: SNMP sections, only for SNMP-enabled devices -- same
  // "section header, table or 'none in this range' line" pattern as the
  // per-check event sections above.
  if (data.snmpEnabled) {
    if (y > 260) {
      doc.addPage();
      y = 18;
    }
    doc.setFontSize(12);
    doc.text("Port up/down events (SNMP)", 14, y);
    y += 4;
    if (data.portEvents.length === 0) {
      doc.setFontSize(9);
      doc.setTextColor(90, 100, 110);
      doc.text("No port down events in this range.", 14, y + 6);
      doc.setTextColor(31, 42, 55);
      y += 14;
    } else {
      (doc as any).autoTable({
        startY: y + 2,
        head: [["Port", "Went down", "Came back up", "Duration"]],
        body: data.portEvents.map((e) => [
          e.portName,
          new Date(e.wentDownAt).toLocaleString(),
          e.cameBackUpAt != null ? new Date(e.cameBackUpAt).toLocaleString() : "still down at end of range",
          fmtDuration(e.durationMs),
        ]),
        styles: { fontSize: 8 },
        headStyles: { fillColor: [92, 107, 122] },
      });
      y = (doc as any).lastAutoTable.finalY + 8;
    }

    if (y > 260) {
      doc.addPage();
      y = 18;
    }
    doc.setFontSize(12);
    doc.text("CPU / memory alerts", 14, y);
    y += 4;
    if (data.hostMetricEvents.length === 0) {
      doc.setFontSize(9);
      doc.setTextColor(90, 100, 110);
      doc.text("No CPU/memory alerts in this range.", 14, y + 6);
      doc.setTextColor(31, 42, 55);
      y += 14;
    } else {
      (doc as any).autoTable({
        startY: y + 2,
        head: [["Type", "Message", "Occurred", "Repeats"]],
        body: data.hostMetricEvents.map((e) => [
          e.label,
          e.message,
          new Date(e.occurredAt).toLocaleString(),
          String(e.count),
        ]),
        styles: { fontSize: 8 },
        headStyles: { fillColor: [92, 107, 122] },
      });
      y = (doc as any).lastAutoTable.finalY + 8;
    }
  }

  const fname = `${data.deviceName.replace(/[^a-z0-9]+/gi, "-").toLowerCase()}-report-${new Date(data.rangeFrom).toISOString().slice(0, 10)}-to-${new Date(data.rangeTo).toISOString().slice(0, 10)}.pdf`;
  doc.save(fname);
}
