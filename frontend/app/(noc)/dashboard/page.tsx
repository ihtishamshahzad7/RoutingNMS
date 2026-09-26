"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "../../../lib/api";
import BackendSync from "./backend-sync";
import { StatCard } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import type { TopoNode, TopoLink } from "../../../components/topology-force";
import styles from "./dashboard.module.css";
import { WidgetShell } from "./WidgetShell";
import { DashboardToolbar } from "./DashboardToolbar";
import { useDashboardLayout, WIDGET_CATALOG, type DashboardWidgetId } from "./dashboardLayout";
import {
  InfraMapWidget,
  PerformanceWidget,
  DeviceStatusWidget,
  IncidentSummaryWidget,
  EventLogWidget,
  AlertsBySourceWidget,
  type RuntimeState,
  type Alert,
  type DeviceHealth,
  type UptimeSummary,
  type DeviceUptime,
} from "./widgets";

const ORG = "tenant-1";
// Devices at or above this latency are counted as "threshold violations" in
// the top stat row -- a real, computed-client-side threshold over real
// latency samples, not a separately-collected metric. 150ms mirrors the
// "elevated" cutoff PerformanceWidget already color-codes its bars against.
const LATENCY_VIOLATION_MS = 150;

type TopologyGraph = { nodes: TopoNode[]; links: TopoLink[]; generatedAt?: string };

export default function DashboardPage() {
  const router = useRouter();
  const [states, setStates] = useState<RuntimeState[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [health, setHealth] = useState<DeviceHealth[]>([]);
  const [healthLoading, setHealthLoading] = useState(true);
  const [uptime, setUptime] = useState<UptimeSummary | null>(null);
  const [graph, setGraph] = useState<TopologyGraph>({ nodes: [], links: [] });
  const [dragState, setDragState] = useState<{ draggedId: DashboardWidgetId | null; overId: DashboardWidgetId | null }>({ draggedId: null, overId: null });

  const order = useDashboardLayout((s) => s.order);
  const hidden = useDashboardLayout((s) => s.hidden);

  const loadHealth = async () => {
    setHealthLoading(true);
    try {
      setHealth(await apiFetch<DeviceHealth[]>(`/devices/health?organizationId=${ORG}`));
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) router.replace("/");
    } finally {
      setHealthLoading(false);
    }
    try {
      setUptime(await apiFetch<UptimeSummary>(`/devices/uptime-summary?organizationId=${ORG}`));
    } catch {
      // Uptime history is a nice-to-have on top of the live health probe
      // above -- an empty/failed fetch just leaves the fleet-status tile
      // and per-device "% 24h" labels blank, nothing else on the page
      // depends on it.
    }
  };

  const uptimeById = useMemo(() => {
    const map: Record<string, DeviceUptime> = {};
    for (const d of uptime?.devices ?? []) map[d.deviceId] = d;
    return map;
  }, [uptime]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const data = await apiFetch<{ olts: RuntimeState[] }>("/olt/runtime");
        if (active) setStates(Array.isArray(data.olts) ? data.olts : []);
      } catch (e) {
        if (active && e instanceof ApiError && e.status === 401) router.replace("/");
      }
      try {
        const g = await apiFetch<TopologyGraph>("/api/topology");
        if (active) setGraph({ nodes: g.nodes ?? [], links: g.links ?? [] });
      } catch {
        // Topology engine may have nothing discovered yet -- InfraMapWidget
        // handles the empty case, no need to surface an error here.
      }
    };
    load();
    loadHealth();
    const t = window.setInterval(() => {
      load();
      loadHealth();
    }, 15000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  useEffect(() => {
    let active = true;
    const load = async () => {
      const results = await Promise.all(
        states.filter((s) => s.oltId).map(async (s) => {
          try {
            return await apiFetch<Alert[]>(`/olts/${encodeURIComponent(s.oltId)}/alerts?limit=10`);
          } catch {
            return [];
          }
        })
      );
      if (active) setAlerts(results.flat().filter((a: Alert) => a.status === "open").slice(0, 20));
    };
    if (states.length) load();
    else setAlerts([]);
    return () => {
      active = false;
    };
  }, [states]);

  const running = states.filter((s) => s.running).length;
  const critical = alerts.filter((a) => a.severity.toLowerCase() === "critical").length;
  const warning = alerts.filter((a) => a.severity.toLowerCase() === "warning").length;
  const healthy = health.filter((d) => d.reachable).length;
  const avgLatency = useMemo(
    () => (health.length ? health.reduce((n, d) => n + d.latencyMs, 0) / health.length : 0),
    [health]
  );
  const violations = useMemo(() => health.filter((d) => d.latencyMs >= LATENCY_VIOLATION_MS).length, [health]);
  void running; // still available for a future "OLT pollers" widget; not in the current grid

  const visibleWidgets = order.filter((id) => !hidden.includes(id));

  const widgetContent: Record<DashboardWidgetId, React.ReactNode> = {
    "infra-map": <InfraMapWidget nodes={graph.nodes} links={graph.links} />,
    performance: <PerformanceWidget health={health} />,
    "device-status": <DeviceStatusWidget health={health} loading={healthLoading} uptimeById={uptimeById} />,
    "incident-summary": <IncidentSummaryWidget alerts={alerts} />,
    "event-log": <EventLogWidget alerts={alerts} />,
    "alerts-by-source": <AlertsBySourceWidget alerts={alerts} />,
  };

  return (
    <main className="mx-auto max-w-[1600px] px-6 py-6">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-4">
        <div>
          <div className="label text-[#8B949E]">Live NOC</div>
          <h1 className="mt-1 text-[22px] font-bold tracking-[-0.5px] text-[#E6EDF3]">Network Operations Center</h1>
          <p className="mt-1 text-xs text-[#8B949E]">Total nodes, active alerts, latency and threshold violations across the whole estate.</p>
        </div>
        <Button variant="secondary" onClick={loadHealth} disabled={healthLoading}>
          {healthLoading ? "Checking devices…" : "Check devices now"}
        </Button>
      </div>

      {/* Top summary bar -- mirrors the screenshot's stat row, all real data */}
      <div className={styles.statRow}>
        <StatCard label="Total Nodes" value={health.length || "—"} sub="Registered devices" accent="text-[#E6EDF3]" />
        <StatCard
          label="Active Alerts"
          value={<span>{critical}<span className="text-[13px] font-normal text-[#8B949E]"> / {warning}</span></span>}
          sub="Critical / Warning"
          accent={critical ? "text-[#F78166]" : "text-[#3FB950]"}
        />
        <StatCard label="Global Latency" value={avgLatency ? avgLatency.toFixed(0) : "—"} unit="ms" sub="Average across reachable devices" accent={avgLatency > LATENCY_VIOLATION_MS ? "text-[#F78166]" : "text-[#3FB950]"} />
        <StatCard label="Threshold Violations" value={violations} sub={`Devices ≥ ${LATENCY_VIOLATION_MS}ms`} accent={violations ? "text-[#D29922]" : "text-[#3FB950]"} />
        <StatCard
          label="Fleet Status"
          value={
            uptime ? (
              <span>
                <span className="text-[#3FB950]">{uptime.up}</span>
                <span className="text-[13px] font-normal text-[#8B949E]"> up</span>
                {" / "}
                <span className="text-[#F78166]">{uptime.down}</span>
                <span className="text-[13px] font-normal text-[#8B949E]"> down</span>
                {" / "}
                <span className="text-[#D29922]">{uptime.warning}</span>
                <span className="text-[13px] font-normal text-[#8B949E]"> warn</span>
              </span>
            ) : (
              "—"
            )
          }
          sub={uptime ? `${uptime.total} devices${uptime.unknown ? ` · ${uptime.unknown} not yet probed` : ""}` : "Based on stored ping history"}
          accent={uptime && uptime.down ? "text-[#F78166]" : "text-[#3FB950]"}
        />
      </div>

      <DashboardToolbar />

      {/* Customizable widget grid -- drag a header to reorder, "..." to
          remove or set a gradient. Layout persists per-browser via
          dashboardLayout.ts (zustand/persist). */}
      <div className={styles.grid}>
        {visibleWidgets.map((id) => {
          const meta = WIDGET_CATALOG.find((w) => w.id === id)!;
          return (
            <div key={id} className={styles[meta.span]}>
              <WidgetShell id={id} title={meta.title} dragState={dragState} setDragState={setDragState}>
                {widgetContent[id]}
              </WidgetShell>
            </div>
          );
        })}
        {visibleWidgets.length === 0 && (
          <div className="col-span-12 flex min-h-32 items-center justify-center rounded-[8px] border border-dashed border-[#21262D] text-xs text-[#8B949E]">
            All widgets removed — use "Add widget" above to bring one back, or "Reset layout".
          </div>
        )}
      </div>

      <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {[
          { href: "/devices", label: "Routers / Switches", cta: "Manage devices →" },
          { href: "/devices", label: "SNMP", cta: "Configure & monitor →" },
          { href: "/olts", label: "OLTs", cta: "Configure →" },
          { href: "/incidents", label: "Incidents", cta: "View alerts →" },
        ].map((l) => (
          <a key={l.label} href={l.href} className="rounded-[6px] bg-[#161B22] border border-[#21262D] p-3 transition-colors duration-100 hover:bg-[#1C2128]">
            <div className="text-xs text-[#8B949E]">{l.label}</div>
            <div className="mt-1 text-xs text-[#58A6FF]">{l.cta}</div>
          </a>
        ))}
      </div>

      <BackendSync />
    </main>
  );
}
