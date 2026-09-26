"use client";

// The six grid modules behind the dashboard's customizable widget system.
// Every widget renders real, already-fetched backend data -- no widget here
// invents a metric RoutingNMS doesn't collect (see the note on "Performance
// Metrics" below for what that ruled out and why).

import Link from "next/link";
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from "recharts";
import { StatusPill } from "../../../components/ui/status-pill";
import { StatusDot } from "../../../components/ui/status-dot";
import { TopologyForce, type TopoNode, type TopoLink } from "../../../components/topology-force";

export type RuntimeState = { oltId: string; running: boolean; startedAt?: string; lastPollAt?: string; lastError?: string; pollCount: number };
export type Alert = { id: number; oltId: string; ponId: string; onuId: string; code: string; severity: string; message: string; status: string; lastSeen: string };
export type DeviceHealth = { id: string; name: string; address: string; deviceType: string; vendor?: string; method: "SNMP" | "TCP"; reachable: boolean; latencyMs: number; checkedAt: string; error?: string };

// Feature 1.3 (Core Dashboard): per-device 24h/7d uptime % derived from
// stored ping_results history (GET /api/v1/devices/uptime-summary), keyed
// by device id -- separate from DeviceHealth, which is a live probe taken
// right now. A device shows "—" here until it has accumulated ICMP history.
export type DeviceUptime = { deviceId: string; name: string; address: string; status: "up" | "down" | "warning" | "unknown"; uptime24h?: number; uptime7d?: number };
export type UptimeSummary = { devices: DeviceUptime[]; up: number; down: number; warning: number; unknown: number; total: number };

function formatTime(v?: string) {
  return v ? new Date(v).toLocaleString() : "—";
}

// --- Module A: Global Infrastructure Map ------------------------------
// Reuses the real, already-shipped LLDP-discovered topology graph
// (/api/topology, feature: Sprint 1 topology engine) and the existing
// TopologyForce D3 canvas from the /topology page -- this is genuine
// discovered infrastructure, not a fabricated world map with invented
// device coordinates.
export function InfraMapWidget({ nodes, links }: { nodes: TopoNode[]; links: TopoLink[] }) {
  if (!nodes.length) {
    return (
      <div className="flex h-full min-h-[200px] items-center justify-center text-center text-xs text-[#8B949E]">
        No discovered topology yet.
        <br />
        <Link href="/topology" className="text-[#58A6FF] hover:underline">Run discovery →</Link>
      </div>
    );
  }
  return <TopologyForce nodes={nodes} links={links} />;
}

// --- Module B: Performance Metrics -------------------------------------
// Top-5 device latency (real, from live health probes) as horizontal bars.
// Deliberately NOT a CPU/Memory chart: RoutingNMS has no HOST-RESOURCES-MIB
// polling anywhere in the codebase (confirmed by the same metric_samples
// writer audit done for the Workspace Topology live-metrics feature), so a
// second chart there would have to be invented data. Left as a flagged
// follow-up rather than faked.
export function PerformanceWidget({ health }: { health: DeviceHealth[] }) {
  const top5 = [...health].sort((a, b) => b.latencyMs - a.latencyMs).slice(0, 5);
  const max = Math.max(1, ...top5.map((d) => d.latencyMs));
  return (
    <div className="space-y-3">
      <div className="label text-[10px] text-[#8B949E]">Top 5 device latency</div>
      {top5.length ? (
        top5.map((d) => (
          <div key={d.id}>
            <div className="mb-1 flex justify-between text-[11px]">
              <span className="truncate text-[#E6EDF3]">{d.name}</span>
              <span className={`mono ${d.latencyMs > 150 ? "text-[#F78166]" : "text-[#8B949E]"}`}>{d.latencyMs.toFixed(1)} ms</span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-[#21262D]">
              <div
                className="h-full rounded-full"
                style={{
                  width: `${(d.latencyMs / max) * 100}%`,
                  background: d.latencyMs > 150 ? "#F78166" : d.latencyMs > 60 ? "#D29922" : "#3FB950",
                }}
              />
            </div>
          </div>
        ))
      ) : (
        <div className="text-xs text-[#8B949E]">No latency samples yet.</div>
      )}
      <div className="mt-2 text-[9px] leading-relaxed text-[#484F58]">
        CPU/Memory utilization requires SNMP HOST-RESOURCES-MIB polling, not yet collected by any RoutingNMS component — not shown here rather than faked.
      </div>
    </div>
  );
}

// --- Module C: Device Status & Availability -----------------------------
export function DeviceStatusWidget({ health, loading, uptimeById }: { health: DeviceHealth[]; loading: boolean; uptimeById?: Record<string, DeviceUptime> }) {
  return (
    <div className="space-y-1.5">
      {health.length ? (
        health.map((d) => {
          const u = uptimeById?.[d.id];
          return (
            <div
              key={d.id}
              className="flex items-center justify-between gap-2 rounded-[6px] border-l-2 bg-[#0D1117] px-2.5 py-1.5"
              style={{ borderLeftColor: d.reachable ? "#3FB950" : "#F78166" }}
            >
              <div className="min-w-0">
                <Link href={`/devices/${d.id}`} className="block truncate text-xs font-medium text-[#58A6FF] hover:underline">{d.name}</Link>
                <span className="mono text-[9px] text-[#484F58]">{d.address}</span>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <span className="mono text-[10px] text-[#8B949E]" title="24h uptime, from stored ping history">
                  {u?.uptime24h != null ? `${u.uptime24h.toFixed(1)}% 24h` : "—"}
                </span>
                <span className="mono text-[10px] text-[#8B949E]">{d.latencyMs.toFixed(0)}ms</span>
                <StatusPill status={d.reachable ? "up" : "down"} label={d.reachable ? "Up" : "Down"} pulse={!d.reachable} />
              </div>
            </div>
          );
        })
      ) : (
        <div className="py-8 text-center text-xs text-[#8B949E]">{loading ? "Checking devices…" : "No registered devices yet."}</div>
      )}
    </div>
  );
}

// --- Module D: Incident Summary ------------------------------------------
export function IncidentSummaryWidget({ alerts }: { alerts: Alert[] }) {
  const critical = alerts.filter((a) => a.severity.toLowerCase() === "critical");
  return (
    <div>
      <div className="mb-2 text-2xl font-bold text-[#F78166]">
        {critical.length} <span className="text-sm font-normal text-[#8B949E]">Critical</span>
      </div>
      <div className="space-y-1.5">
        {critical.length ? (
          critical.slice(0, 6).map((a) => (
            <div key={a.id} className="truncate rounded-[6px] bg-[#0D1117] px-2.5 py-1.5 text-[11px] text-[#E6EDF3]" title={a.message}>
              <StatusDot status="critical" /> <span className="ml-1">{a.message}</span>
            </div>
          ))
        ) : (
          <div className="text-xs text-[#8B949E]">No critical incidents open.</div>
        )}
      </div>
    </div>
  );
}

// --- Module E: Live Event Log ---------------------------------------------
export function EventLogWidget({ alerts }: { alerts: Alert[] }) {
  const rows = [...alerts].sort((a, b) => new Date(b.lastSeen).getTime() - new Date(a.lastSeen).getTime()).slice(0, 25);
  return (
    <table className="tbl w-full text-left text-[10px]">
      <thead><tr><th>Time</th><th>Status</th><th>Message</th></tr></thead>
      <tbody>
        {rows.length ? rows.map((a) => (
          <tr key={a.id}>
            <td className="mono whitespace-nowrap text-[#484F58]">{formatTime(a.lastSeen)}</td>
            <td><StatusPill status={a.severity} label={a.severity} /></td>
            <td className="mono max-w-[260px] truncate text-[#E6EDF3]" title={a.message}>{a.message}</td>
          </tr>
        )) : <tr><td colSpan={3} className="py-8 text-center text-[#8B949E]">No events.</td></tr>}
      </tbody>
    </table>
  );
}

// --- Module F (replaces "Alerts by Source Type"): Alerts by OLT -----------
// The screenshot's "Alerts by Source Type" buckets alerts into
// System/Application/Network/Storage -- RoutingNMS alerts don't carry that
// taxonomy. Grouping by oltId is the real, honest equivalent grouping this
// data actually supports.
const DONUT_COLORS = ["#58A6FF", "#3FB950", "#D29922", "#F78166", "#A371F7", "#8B949E"];
export function AlertsBySourceWidget({ alerts }: { alerts: Alert[] }) {
  const counts = new Map<string, number>();
  for (const a of alerts) counts.set(a.oltId || "unassigned", (counts.get(a.oltId || "unassigned") ?? 0) + 1);
  const data = [...counts.entries()].map(([name, value]) => ({ name, value }));

  if (!data.length) return <div className="py-8 text-center text-xs text-[#8B949E]">No active alerts.</div>;

  return (
    <div className="flex h-full items-center gap-3">
      <div className="h-[160px] w-[160px] shrink-0">
        <ResponsiveContainer>
          <PieChart>
            <Pie data={data} dataKey="value" nameKey="name" innerRadius={45} outerRadius={72} paddingAngle={2}>
              {data.map((_, i) => <Cell key={i} fill={DONUT_COLORS[i % DONUT_COLORS.length]} />)}
            </Pie>
            <Tooltip contentStyle={{ background: "#0D1117", border: "1px solid #21262D", fontSize: 11 }} />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="min-w-0 flex-1 space-y-1">
        {data.map((d, i) => (
          <div key={d.name} className="flex items-center gap-1.5 text-[11px]">
            <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: DONUT_COLORS[i % DONUT_COLORS.length] }} />
            <span className="mono truncate text-[#8B949E]">{d.name}</span>
            <span className="ml-auto text-[#E6EDF3]">{d.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
