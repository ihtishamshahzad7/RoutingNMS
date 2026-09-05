"use client";

// Reachability -- a unified, Uptime-Kuma-style monitor list: every device's
// ICMP ping, HTTP(S) check and traceroute action in one screen, with a live
// RTT sparkline per row (Kuma only shows a status pill; this also charts the
// last hour of ping history inline, and adds sortable/filterable summary
// counts on top). Deliberately reuses existing endpoints instead of adding a
// new bulk backend route:
//   - GET /devices?organizationId=...        device inventory + which checks
//                                             are enabled per device
//   - GET /ping/{id}/live                    latest ICMP result + last 60
//                                             probes in one call (status +
//                                             sparkline data, no extra fetch)
//   - GET /metrics?...&metric=http_up&metric=http_latency_ms
//                                             latest HTTP check status
// Device counts here are small/moderate, so one call per device per check
// (in parallel) is simpler and cheaper to maintain than a new aggregate API.

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { StatusPill } from "../../../components/ui/status-pill";
import { Sparkline } from "../../../components/ui/sparkline";
import { groupSections, GroupSectionRows, DeviceGroup, GroupMember } from "../../../lib/device-groups";

const ORG = "tenant-1";

type Device = {
  id: string;
  name: string;
  address: string;
  deviceType: string;
  vendor?: string;
  enabled: boolean;
  icmpEnabled: boolean;
  httpCheckEnabled: boolean;
  httpUrl?: string;
  dnsEnabled: boolean;
  dnsHostname?: string;
  pushEnabled: boolean;
  sshEnabled: boolean;
  sshPort?: number;
  telnetEnabled: boolean;
  telnetPort?: number;
};

type PingResult = { reachable: boolean; rttMs: number; lossPct: number; probedAt: string; error?: string };
type PingLive = { live: PingResult; history: PingResult[] };
type MetricPoint = { timestamp: string; value: number };
type MetricSeries = { metric: string; points: MetricPoint[] };

type RowState = "loading" | "up" | "down" | "unmonitored";
type DNSLive = { live: { resolved: boolean; latencyMs: number; error?: string } };
type ReachLive = { live: { reachable: boolean; latencyMs: number; error?: string } };

function SSHCell({ device }: { device: Device }) {
  const [data, setData] = useState<ReachLive | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!device.sshEnabled) return;
    let active = true;
    apiFetch<ReachLive>(`/ssh/${device.id}/live`)
      .then((d) => { if (active) setData(d); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [device.id, device.sshEnabled]);

  if (!device.sshEnabled) return <span className="text-[10px] text-[#484F58]">—</span>;
  if (error) return <span className="text-[10px] text-[#F78166]">unavailable</span>;
  if (!data?.live) return <span className="text-[10px] text-[#8B949E]">…</span>;

  const up = data.live.reachable;
  return (
    <div className="w-20">
      <StatusPill status={up ? "up" : "down"} label={up ? "Reachable" : "Down"} />
      <div className="mt-1 font-mono text-[10px] text-[#8B949E]">port {device.sshPort || 22}</div>
    </div>
  );
}

function TelnetCell({ device }: { device: Device }) {
  const [data, setData] = useState<ReachLive | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!device.telnetEnabled) return;
    let active = true;
    apiFetch<ReachLive>(`/telnet/${device.id}/live`)
      .then((d) => { if (active) setData(d); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [device.id, device.telnetEnabled]);

  if (!device.telnetEnabled) return <span className="text-[10px] text-[#484F58]">—</span>;
  if (error) return <span className="text-[10px] text-[#F78166]">unavailable</span>;
  if (!data?.live) return <span className="text-[10px] text-[#8B949E]">…</span>;

  const up = data.live.reachable;
  return (
    <div className="w-20">
      <StatusPill status={up ? "up" : "down"} label={up ? "Reachable" : "Down"} />
      <div className="mt-1 font-mono text-[10px] text-[#8B949E]">port {device.telnetPort || 23}</div>
    </div>
  );
}

function DnsCell({ device }: { device: Device }) {
  const [data, setData] = useState<DNSLive | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!device.dnsEnabled) return;
    let active = true;
    apiFetch<DNSLive>(`/dns/${device.id}/live`)
      .then((d) => { if (active) setData(d); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [device.id, device.dnsEnabled]);

  if (!device.dnsEnabled) return <span className="text-[10px] text-[#484F58]">—</span>;
  if (error) return <span className="text-[10px] text-[#F78166]">unavailable</span>;
  if (!data?.live) return <span className="text-[10px] text-[#8B949E]">…</span>;

  const up = data.live.resolved;
  return (
    <div className="w-20">
      <StatusPill status={up ? "up" : "down"} label={up ? "Resolves" : "Failing"} />
      <div className="mt-1 font-mono text-[10px] text-[#8B949E]">{device.dnsHostname}</div>
    </div>
  );
}

function PushCell({ device }: { device: Device }) {
  const [series, setSeries] = useState<MetricSeries[] | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!device.pushEnabled) return;
    let active = true;
    apiFetch<MetricSeries[]>(
      `/metrics?subjectType=device&subjectId=${encodeURIComponent(device.id)}&metric=push_up&since=6h`
    )
      .then((s) => { if (active) setSeries(s); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [device.id, device.pushEnabled]);

  if (!device.pushEnabled) return <span className="text-[10px] text-[#484F58]">—</span>;
  if (error) return <span className="text-[10px] text-[#F78166]">unavailable</span>;
  if (!series) return <span className="text-[10px] text-[#8B949E]">…</span>;

  const upSeries = series.find((s) => s.metric === "push_up");
  const lastUp = upSeries?.points[upSeries.points.length - 1]?.value;
  const known = lastUp !== undefined;
  const up = lastUp === 1;
  return <StatusPill status={known ? (up ? "up" : "down") : "unknown"} label={known ? (up ? "Up" : "No heartbeat") : "No data"} />;
}

function PingCell({ device }: { device: Device }) {
  const [data, setData] = useState<PingLive | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!device.icmpEnabled) return;
    let active = true;
    apiFetch<PingLive>(`/ping/${device.id}/live`)
      .then((d) => { if (active) setData(d); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [device.id, device.icmpEnabled]);

  if (!device.icmpEnabled) {
    return <StatusPill status="unknown" label="Not monitored" />;
  }
  if (error) return <span className="text-[10px] text-[#F78166]">unavailable</span>;
  if (!data) return <span className="text-[10px] text-[#8B949E]">…</span>;

  const history = data.history.filter((p) => p.reachable).map((p) => p.rttMs);
  const up = data.live.reachable;

  return (
    <div className="flex items-center gap-3">
      <div className="w-16">
        <StatusPill status={up ? "up" : "down"} label={up ? "Up" : "Down"} pulse={up} />
        <div className="mt-1 font-mono text-[10px] text-[#8B949E]">
          {up ? `${data.live.rttMs.toFixed(1)}ms` : data.live.error ? "timeout" : "—"}
        </div>
      </div>
      <Sparkline points={history} width={140} height={28} stroke={up ? "#3FB950" : "#F78166"} />
    </div>
  );
}

function HttpCell({ device }: { device: Device }) {
  const [series, setSeries] = useState<MetricSeries[] | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (!device.httpCheckEnabled) return;
    let active = true;
    apiFetch<MetricSeries[]>(
      `/metrics?subjectType=device&subjectId=${encodeURIComponent(device.id)}&metric=http_up&metric=http_latency_ms&since=6h`
    )
      .then((s) => { if (active) setSeries(s); })
      .catch(() => { if (active) setError(true); });
    return () => { active = false; };
  }, [device.id, device.httpCheckEnabled]);

  if (!device.httpCheckEnabled) {
    return <span className="text-[10px] text-[#484F58]">—</span>;
  }
  if (error) return <span className="text-[10px] text-[#F78166]">unavailable</span>;
  if (!series) return <span className="text-[10px] text-[#8B949E]">…</span>;

  const upSeries = series.find((s) => s.metric === "http_up");
  const latSeries = series.find((s) => s.metric === "http_latency_ms");
  const lastUp = upSeries?.points[upSeries.points.length - 1]?.value;
  const up = lastUp === 1;
  const known = lastUp !== undefined;
  const latencyPoints = (latSeries?.points ?? []).map((p) => p.value);
  const lastLatency = latencyPoints[latencyPoints.length - 1];

  return (
    <div className="flex items-center gap-3">
      <div className="w-16">
        <StatusPill status={known ? (up ? "up" : "down") : "unknown"} label={known ? (up ? "Up" : "Down") : "No data"} />
        {lastLatency !== undefined && (
          <div className="mt-1 font-mono text-[10px] text-[#8B949E]">{lastLatency.toFixed(0)}ms</div>
        )}
      </div>
      {latencyPoints.length > 0 && (
        <Sparkline points={latencyPoints} width={100} height={28} stroke={up ? "#58A6FF" : "#F78166"} />
      )}
    </div>
  );
}

/** Tracks a device's live ping state (up/down/unmonitored) purely for the
 * summary counts and status filter -- reusing the same /ping/{id}/live call
 * PingCell makes, so it's one request per device, not two. */
function useRowState(device: Device): RowState {
  const [state, setState] = useState<RowState>(device.icmpEnabled ? "loading" : "unmonitored");
  useEffect(() => {
    if (!device.icmpEnabled) { setState("unmonitored"); return; }
    let active = true;
    apiFetch<PingLive>(`/ping/${device.id}/live`)
      .then((d) => { if (active) setState(d.live.reachable ? "up" : "down"); })
      .catch(() => { if (active) setState("down"); });
    return () => { active = false; };
  }, [device.id, device.icmpEnabled]);
  return state;
}

function DeviceRow({ device }: { device: Device }) {
  return (
    <tr className="border-b border-[#21262D] last:border-0">
      <td className="py-3 pl-4 pr-3 align-top">
        <Link href={`/devices/${device.id}`} className="font-medium text-[#58A6FF] hover:underline">
          {device.name}
        </Link>
        <div className="mt-0.5 font-mono text-[10px] text-[#8B949E]">{device.address}</div>
        <div className="mt-0.5 text-[9px] uppercase tracking-[0.06em] text-[#484F58]">
          {device.deviceType}{device.vendor ? ` · ${device.vendor}` : ""}
        </div>
      </td>
      <td className="py-3 pr-3 align-top">
        <PingCell device={device} />
      </td>
      <td className="py-3 pr-3 align-top">
        <HttpCell device={device} />
      </td>
      <td className="py-3 pr-3 align-top">
        <DnsCell device={device} />
      </td>
      <td className="py-3 pr-3 align-top">
        <SSHCell device={device} />
      </td>
      <td className="py-3 pr-3 align-top">
        <TelnetCell device={device} />
      </td>
      <td className="py-3 pr-3 align-top">
        <PushCell device={device} />
      </td>
      <td className="py-3 pr-4 align-top">
        <Link
          href={`/devices/${device.id}#traceroute`}
          className="inline-flex items-center gap-1.5 rounded-[5px] border border-[#30363D] bg-[#21262D] px-3 py-1.5 text-[11px] text-[#E6EDF3] transition-colors duration-100 hover:bg-[#1C2128]"
        >
          Traceroute
        </Link>
      </td>
    </tr>
  );
}

type StatusFilter = "all" | "up" | "down" | "unmonitored";
type SortKey = "name" | "type";

export default function ReachabilityPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [filter, setFilter] = useState<StatusFilter>("all");
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [query, setQuery] = useState("");
  // Bumps every 30s to force PingCell/HttpCell/useRowState to re-fetch,
  // keeping the whole board "live" the way Kuma's monitor list is.
  const [tick, setTick] = useState(0);
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [memberOf, setMemberOf] = useState<Record<string, number>>({});
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());

  async function load() {
    setLoading(true);
    try {
      setDevices(await apiFetch<Device[]>(`/devices?organizationId=${ORG}`));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load devices.");
    } finally {
      setLoading(false);
    }
  }
  async function loadGroups() {
    try {
      const [gs, ms] = await Promise.all([
        apiFetch<DeviceGroup[]>(`/device-groups?tenantId=${ORG}`),
        apiFetch<GroupMember[]>("/device-groups/members"),
      ]);
      setGroups(gs);
      const map: Record<string, number> = {};
      ms.filter((m) => m.subjectType === "device").forEach((m) => { map[m.subjectId] = m.groupId; });
      setMemberOf(map);
    } catch {
      // Groups are a nice-to-have overlay on the board -- a failure here
      // shouldn't block rendering live status.
    }
  }
  useEffect(() => { load(); loadGroups(); }, []);
  useEffect(() => {
    const t = window.setInterval(() => setTick((n) => n + 1), 30000);
    return () => window.clearInterval(t);
  }, []);
  function toggleGroupCollapsed(key: string) {
    setCollapsedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  }

  const filtered = useMemo(() => {
    let list = devices;
    if (query.trim()) {
      const q = query.trim().toLowerCase();
      list = list.filter((d) => d.name.toLowerCase().includes(q) || d.address.toLowerCase().includes(q));
    }
    list = [...list].sort((a, b) =>
      sortKey === "name" ? a.name.localeCompare(b.name) : a.deviceType.localeCompare(b.deviceType) || a.name.localeCompare(b.name)
    );
    return list;
  }, [devices, query, sortKey]);

  const counts = useMemo(() => {
    const monitored = devices.filter((d) => d.icmpEnabled);
    return {
      total: devices.length,
      monitored: monitored.length,
      httpChecks: devices.filter((d) => d.httpCheckEnabled).length,
      unmonitored: devices.length - monitored.length,
    };
  }, [devices]);

  return (
    <main className="mx-auto max-w-7xl px-6 py-8" key={tick}>
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">LIVE NOC / UNIFIED MONITORING</div>
        <h1 className="mt-2 text-3xl font-bold text-[#E6EDF3]">Reachability</h1>
        <p className="mt-2 max-w-3xl text-sm text-[#8B949E]">
          Every device&apos;s ping, HTTP check and traceroute in one board — Uptime Kuma&apos;s monitor list,
          extended with a live RTT chart per row instead of a bare status pill. Refreshes every 30s.
        </p>
      </div>

      {message && (
        <div className="mb-5 rounded-[8px] border border-[#672525] bg-[#2D1212] px-4 py-3 text-sm text-[#F78166]">{message}</div>
      )}

      <div className="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <SummaryTile label="Devices" value={counts.total} onClick={() => setFilter("all")} active={filter === "all"} />
        <SummaryTile label="ICMP monitored" value={counts.monitored} accent="text-[#3FB950]" onClick={() => setFilter("up")} active={filter === "up"} />
        <SummaryTile label="HTTP checks" value={counts.httpChecks} accent="text-[#58A6FF]" onClick={() => setFilter("down")} active={filter === "down"} />
        <SummaryTile label="Unmonitored" value={counts.unmonitored} accent="text-[#8B949E]" onClick={() => setFilter("unmonitored")} active={filter === "unmonitored"} />
      </div>

      <Card
        title="Monitor board"
        headerRight={
          <div className="flex items-center gap-2">
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter by name or address…"
              className="w-52 rounded-[5px] border border-[#30363D] bg-[#0D1117] px-2.5 py-1.5 text-[11px] text-[#E6EDF3] outline-none placeholder:text-[#484F58] focus:border-[#58A6FF]"
            />
            <select
              value={sortKey}
              onChange={(e) => setSortKey(e.target.value as SortKey)}
              className="rounded-[5px] border border-[#30363D] bg-[#0D1117] px-2 py-1.5 text-[11px] text-[#E6EDF3] outline-none focus:border-[#58A6FF]"
            >
              <option value="name">Sort: Name</option>
              <option value="type">Sort: Type</option>
            </select>
            <button onClick={load} className="rounded-[5px] border border-[#30363D] bg-[#21262D] px-3 py-1.5 text-[11px] text-[#E6EDF3] transition-colors duration-100 hover:bg-[#1C2128]">
              Refresh
            </button>
          </div>
        }
      >
        <FilterHint filter={filter} onClear={() => setFilter("all")} />
        <div className="overflow-x-auto">
          <table className="tbl w-full min-w-[1080px] text-left text-xs">
            <thead>
              <tr>
                <th>Device</th>
                <th>ICMP / Ping</th>
                <th>HTTP check</th>
                <th>DNS check</th>
                <th>SSH</th>
                <th>Telnet</th>
                <th>Push heartbeat</th>
                <th>Trace</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={8} className="py-10 text-center text-[#8B949E]">Loading reachability board…</td></tr>
              ) : filtered.length ? (
                groupSections(filtered, groups, memberOf).map((section) => (
                  <GroupSectionRows
                    key={section.key}
                    section={section}
                    collapsed={collapsedGroups.has(section.key)}
                    onToggle={() => toggleGroupCollapsed(section.key)}
                    render={(d) => <FilteredRow key={d.id} device={d} filter={filter} />}
                  />
                ))
              ) : (
                <tr><td colSpan={8} className="py-10 text-center text-[#8B949E]">No devices match.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </Card>
    </main>
  );
}

function SummaryTile({
  label, value, accent = "text-[#E6EDF3]", onClick, active,
}: { label: string; value: number; accent?: string; onClick: () => void; active: boolean }) {
  return (
    <button
      onClick={onClick}
      className={`rounded-[8px] border px-4 py-3 text-left transition-colors duration-100 ${
        active ? "border-[#58A6FF] bg-[#161B22]" : "border-[#21262D] bg-[#161B22] hover:bg-[#1C2128]"
      }`}
    >
      <div className="text-[10px] uppercase tracking-[0.08em] text-[#8B949E]">{label}</div>
      <div className={`mt-1 text-2xl font-bold ${accent}`}>{value}</div>
    </button>
  );
}

function FilterHint({ filter, onClear }: { filter: StatusFilter; onClear: () => void }) {
  if (filter === "all") return null;
  const labels: Record<StatusFilter, string> = { all: "", up: "ICMP-monitored", down: "with HTTP checks", unmonitored: "unmonitored" };
  return (
    <div className="flex items-center justify-between border-b border-[#21262D] px-4 py-2 text-[10px] text-[#8B949E]">
      <span>Showing: {labels[filter]}</span>
      <button onClick={onClear} className="text-[#58A6FF] hover:underline">Clear</button>
    </div>
  );
}

/** Wraps DeviceRow to honor the summary-tile quick filters (up/down/unmonitored)
 * without duplicating the row's own live-status fetch. */
function FilteredRow({ device, filter }: { device: Device; filter: StatusFilter }) {
  const state = useRowState(device);
  if (filter === "unmonitored" && state !== "unmonitored") return null;
  if (filter === "up" && !device.icmpEnabled) return null;
  if (filter === "down" && !device.httpCheckEnabled) return null;
  return <DeviceRow device={device} />;
}
