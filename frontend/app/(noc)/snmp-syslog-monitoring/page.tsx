"use client";

// Events (SNMP & Syslog Monitoring) page (6-page rebuild, item 3.3b).
// Consumes the unified events backend from item 3.3a (GET /api/v1/events):
// device down/up, interface down/up, and classified syslog messages, all
// deduplicated server-side into rows carrying a repeat count and
// first/last-seen. This page is a triage view, not a log table: a live
// counts strip up top, severity-first + recency ordering, color that
// carries meaning at a glance, and one click through to the affected
// device (deep-linked to the specific interface when the event names one).

import { ReactNode, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  ChevronDown,
  ChevronRight,
  Cpu,
  List,
  Radar,
  Rows3,
  RotateCw,
  Search,
} from "lucide-react";
import { apiFetch, ApiError } from "../../../lib/api";
import { EngPanel, EngInput, EngSelect } from "../../../components/ui/engineer";
import type { DeviceGroup, GroupMember } from "../../../lib/device-groups";

const ORG = "tenant-1";

type Severity = "critical" | "warning" | "info";

type EventRecord = {
  id: number;
  deviceId?: string;
  groupId?: string;
  eventType: string;
  severity: Severity;
  message: string;
  source: "snmp" | "syslog" | "poller";
  refTable?: string;
  refId?: string;
  count: number;
  firstSeen: string;
  lastSeen: string;
};
type EventsResponse = { items: EventRecord[]; limit: number; offset: number; hasMore: boolean };
type DeviceLite = { id: string; name: string };

const SEV_COLOR: Record<Severity, string> = { critical: "#C4362D", warning: "#C77700", info: "#2E7BF6" };
const SEV_LABEL: Record<Severity, string> = { critical: "Critical", warning: "Warning", info: "Info" };

// Plain-language label per event type -- the backend's own `message` field
// is already readable ("device1: interface ether3 is down"), but repeats
// the device name we show in its own column and uses full-sentence
// grammar; this trims it to the terse "Interface ether3 down" style the
// spec asks for, falling back to a generic label (with the full message
// kept as secondary detail) for event types that don't name a port.
const EVENT_TYPE_LABEL: Record<string, string> = {
  device_down: "Device down",
  device_up: "Device back up",
  interface_down: "Interface down",
  interface_up: "Interface up",
  loop_detected: "Loop detected",
  mac_flapping: "MAC flapping",
  stp_topology_change: "STP topology change",
  auth_failure: "Authentication failure",
  device_reboot: "Device reboot",
  syslog: "Syslog message",
};

const interfaceRe = /interface\s+(\S+)\s+is\s+(up|down)/i;
const devicePrefixRe = /^[^:]+:\s*/;

/** Short, human primary line for a row -- "Interface ether3 down" instead
 * of the backend's full-sentence message. Interface events get the port
 * name spliced in; everything else falls back to the type label alone
 * (the full message is shown separately as secondary detail). */
function primaryLabel(ev: EventRecord): string {
  const m = ev.message.match(interfaceRe);
  if (m && (ev.eventType === "interface_down" || ev.eventType === "interface_up")) {
    return `Interface ${m[1]} ${m[2]}`;
  }
  return EVENT_TYPE_LABEL[ev.eventType] ?? ev.eventType;
}

/** The interface name/target to deep-link into the device page with, or
 * null when this event isn't about a specific interface. */
function interfaceTarget(ev: EventRecord): string | null {
  const m = ev.message.match(interfaceRe);
  return m ? m[1] : null;
}

/** Secondary detail line -- the backend's full message, with a redundant
 * leading "device name: " prefix stripped since the device is already its
 * own column. Always shown for non-interface event types (this is where a
 * raw syslog line or the loop/flap/auth detail actually lives); for
 * interface events it's only shown when it adds something past the port
 * name already in the primary line. */
function secondaryDetail(ev: EventRecord): string | null {
  // Device and interface up/down events are already fully expressed by the
  // device column + primaryLabel() -- showing the raw message under them
  // too would just repeat "Router1 is down" beneath "Router1 · Device
  // down". Every other type (loop/flap/topology/auth/reboot/syslog) has no
  // such redundancy, so its message is real added detail worth showing.
  if (["interface_down", "interface_up", "device_down", "device_up"].includes(ev.eventType)) {
    return null;
  }
  return ev.message.replace(devicePrefixRe, "");
}

function timeAgo(iso: string) {
  try {
    const d = +new Date(iso);
    if (!d) return "";
    const s = Math.max(0, Math.floor((Date.now() - d) / 1000));
    if (s < 5) return "just now";
    if (s < 60) return `${s}s ago`;
    if (s < 3600) return `${Math.floor(s / 60)}m ago`;
    if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
    return `${Math.floor(s / 86400)}d ago`;
  } catch {
    return "";
  }
}

function fullTimestamp(iso: string) {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

type TimeRange = { key: string; label: string; ms: number | null };
const TIME_RANGES: TimeRange[] = [
  { key: "1h", label: "Last hour", ms: 60 * 60 * 1000 },
  { key: "24h", label: "Last 24 hours", ms: 24 * 60 * 60 * 1000 },
  { key: "7d", label: "Last 7 days", ms: 7 * 24 * 60 * 60 * 1000 },
  { key: "all", label: "All time", ms: null },
];

const EVENT_TYPES = Object.keys(EVENT_TYPE_LABEL);

function StatCard({ label, value, color, icon }: { label: string; value: number; color: string; icon: ReactNode }) {
  return (
    <div className="flex flex-1 items-center gap-3 rounded-[4px] border border-[#DCE1E8] bg-white px-4 py-3">
      <span
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full"
        style={{ background: `${color}1A`, color }}
      >
        {icon}
      </span>
      <div className="min-w-0">
        <div className="text-[22px] font-semibold leading-none" style={{ color }}>
          {value > 999 ? "999+" : value}
        </div>
        <div className="mt-1 text-[11px] font-medium uppercase tracking-wide text-[#5C6B7A]">{label}</div>
      </div>
    </div>
  );
}

function RowSkeleton() {
  return (
    <div className="flex animate-pulse items-center gap-3 px-3 py-2.5">
      <div className="h-8 w-[3px] shrink-0 rounded-full bg-[#EEF1F4]" />
      <div className="min-w-0 flex-1">
        <div className="h-[13px] w-48 rounded-[3px] bg-[#EEF1F4]" />
        <div className="mt-1.5 h-[11px] w-32 rounded-[3px] bg-[#EEF1F4]" />
      </div>
      <div className="h-[11px] w-16 shrink-0 rounded-[3px] bg-[#EEF1F4]" />
    </div>
  );
}

export default function EventsPage() {
  const [events, setEvents] = useState<EventRecord[]>([]);
  const [summaryEvents, setSummaryEvents] = useState<EventRecord[]>([]);
  const [devices, setDevices] = useState<DeviceLite[]>([]);
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [memberOf, setMemberOf] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [hasMore, setHasMore] = useState(false);

  const [search, setSearch] = useState("");
  const [severity, setSeverity] = useState("");
  const [deviceId, setDeviceId] = useState("");
  const [groupId, setGroupId] = useState("");
  const [eventType, setEventType] = useState("");
  const [timeRange, setTimeRange] = useState("24h");
  const [grouped, setGrouped] = useState(false);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [limit, setLimit] = useState(100);

  const deviceById = useMemo(() => {
    const m: Record<string, DeviceLite> = {};
    devices.forEach((d) => { m[d.id] = d; });
    return m;
  }, [devices]);
  const groupById = useMemo(() => {
    const m: Record<number, DeviceGroup> = {};
    groups.forEach((g) => { m[g.id] = g; });
    return m;
  }, [groups]);
  function groupNameForDevice(devId?: string): string | null {
    if (!devId) return null;
    const gid = memberOf[devId];
    if (gid === undefined) return null;
    return groupById[gid]?.name ?? null;
  }

  async function loadReference() {
    try {
      const [ds, gs, ms] = await Promise.all([
        apiFetch<DeviceLite[]>(`/devices?organizationId=${ORG}`),
        apiFetch<DeviceGroup[]>(`/device-groups?tenantId=${ORG}`),
        apiFetch<GroupMember[]>("/device-groups/members"),
      ]);
      setDevices(ds);
      setGroups(gs);
      const map: Record<string, number> = {};
      ms.filter((m) => m.subjectType === "device").forEach((m) => { map[m.subjectId] = m.groupId; });
      setMemberOf(map);
    } catch {
      // Reference data (names/groups) is a display nicety -- if it fails
      // the event list below still loads and just shows raw ids.
    }
  }

  async function loadEvents() {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      params.set("limit", String(limit));
      if (severity) params.set("severity", severity);
      if (deviceId) params.set("device", deviceId);
      if (groupId) params.set("group", groupId);
      if (eventType) params.set("eventType", eventType);
      if (search.trim()) params.set("text", search.trim());
      const range = TIME_RANGES.find((r) => r.key === timeRange);
      if (range?.ms) params.set("from", new Date(Date.now() - range.ms).toISOString());
      const res = await apiFetch<EventsResponse>(`/events?${params.toString()}`);
      setEvents(res.items);
      setHasMore(res.hasMore);
      setError("");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unable to reach the events API.");
    } finally {
      setLoading(false);
    }
  }

  // The top strip always reflects true current state regardless of the
  // list's own filters/time range below -- a separate, unfiltered (beyond
  // a fixed 24h lookback so the query stays cheap) fetch.
  async function loadSummary() {
    try {
      const res = await apiFetch<EventsResponse>(
        `/events?limit=500&from=${encodeURIComponent(new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString())}`
      );
      setSummaryEvents(res.items);
    } catch {
      // Non-fatal -- the strip just won't update this tick.
    }
  }

  useEffect(() => {
    loadReference();
  }, []);
  useEffect(() => {
    loadEvents();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [severity, deviceId, groupId, eventType, timeRange, search, limit]);
  useEffect(() => {
    loadSummary();
    const t = window.setInterval(loadSummary, 15000);
    return () => window.clearInterval(t);
  }, []);
  // The visible list also refreshes on the same cadence, so "live" means
  // both the strip and the rows update without a reload.
  useEffect(() => {
    const t = window.setInterval(loadEvents, 15000);
    return () => window.clearInterval(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [severity, deviceId, groupId, eventType, timeRange, search, limit]);

  const counts = useMemo(() => {
    let critical = 0;
    let warning = 0;
    // "Unresolved" -- best-effort from the data we have: no event in this
    // schema carries a resolved/acked flag, so this counts device/interface
    // targets whose *latest* known state (by last_seen, across the summary
    // window) is a "down" event with no later "up" event for that same
    // target. Everything else (loop/flap/topology/auth/reboot/syslog) is a
    // point-in-time occurrence, not an ongoing state, so it isn't counted
    // here even at critical severity -- it still shows in the Critical tile.
    const latestStateByTarget = new Map<string, "up" | "down">();
    const sorted = [...summaryEvents].sort((a, b) => +new Date(a.lastSeen) - +new Date(b.lastSeen));
    for (const ev of sorted) {
      if (ev.severity === "critical") critical++;
      else if (ev.severity === "warning") warning++;
      if (ev.eventType === "device_down" || ev.eventType === "device_up") {
        if (!ev.deviceId) continue;
        latestStateByTarget.set(`d:${ev.deviceId}`, ev.eventType === "device_down" ? "down" : "up");
      } else if (ev.eventType === "interface_down" || ev.eventType === "interface_up") {
        const target = interfaceTarget(ev);
        if (!ev.deviceId || !target) continue;
        latestStateByTarget.set(`i:${ev.deviceId}:${target}`, ev.eventType === "interface_down" ? "down" : "up");
      }
    }
    let unresolved = 0;
    latestStateByTarget.forEach((state) => { if (state === "down") unresolved++; });
    return { critical, warning, unresolved };
  }, [summaryEvents]);

  // Severity-and-unresolved-first, then most recent -- critical rows
  // (and any row still in an unresolved down state) float to the top
  // regardless of what the backend's own last_seen ordering gave us.
  const sortedEvents = useMemo(() => {
    const severityRank: Record<Severity, number> = { critical: 0, warning: 1, info: 2 };
    return [...events].sort((a, b) => {
      const r = severityRank[a.severity] - severityRank[b.severity];
      if (r !== 0) return r;
      return +new Date(b.lastSeen) - +new Date(a.lastSeen);
    });
  }, [events]);

  const groupedSections = useMemo(() => {
    if (!grouped) return null;
    const byDevice = new Map<string, EventRecord[]>();
    const noDevice: EventRecord[] = [];
    for (const ev of sortedEvents) {
      if (!ev.deviceId) { noDevice.push(ev); continue; }
      const bucket = byDevice.get(ev.deviceId);
      if (bucket) bucket.push(ev); else byDevice.set(ev.deviceId, [ev]);
    }
    const sections = [...byDevice.entries()]
      .map(([devId, items]) => ({
        key: devId,
        name: deviceById[devId]?.name ?? `Device ${devId}`,
        items,
        worstSeverity: items.reduce<Severity>((worst, ev) => {
          const rank: Record<Severity, number> = { critical: 0, warning: 1, info: 2 };
          return rank[ev.severity] < rank[worst] ? ev.severity : worst;
        }, "info"),
      }))
      .sort((a, b) => {
        const rank: Record<Severity, number> = { critical: 0, warning: 1, info: 2 };
        return rank[a.worstSeverity] - rank[b.worstSeverity] || a.name.localeCompare(b.name);
      });
    if (noDevice.length) sections.push({ key: "none", name: "Unmapped source", items: noDevice, worstSeverity: "info" });
    return sections;
  }, [grouped, sortedEvents, deviceById]);

  function toggleSection(key: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  }

  function EventRow({ ev }: { ev: EventRecord }) {
    const group = groupNameForDevice(ev.deviceId);
    const deviceName = ev.deviceId ? deviceById[ev.deviceId]?.name ?? `Device ${ev.deviceId}` : null;
    const target = interfaceTarget(ev);
    const href = ev.deviceId ? `/devices/${ev.deviceId}${target ? `?highlight=${encodeURIComponent(target)}` : ""}` : null;
    const detail = secondaryDetail(ev);

    const content = (
      <div className="flex items-center gap-3 px-3 py-2.5">
        <span className="h-8 w-[3px] shrink-0 rounded-full" style={{ background: SEV_COLOR[ev.severity] }} />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            {deviceName && <span className="text-[13px] font-medium text-[#1F2A37]">{deviceName}</span>}
            {group && <span className="text-[11px] text-[#8A96A3]">({group})</span>}
            <span className="text-[13px] text-[#1F2A37]">{primaryLabel(ev)}</span>
            {ev.count > 1 && (
              <span
                title={`First seen ${fullTimestamp(ev.firstSeen)}\nLast seen ${fullTimestamp(ev.lastSeen)}`}
                className="inline-flex items-center rounded-[3px] bg-[#F4F6F9] px-1.5 py-[1px] text-[10px] font-semibold text-[#5C6B7A]"
              >
                ×{ev.count}
              </span>
            )}
          </div>
          {detail && <div className="mt-0.5 truncate text-[11px] text-[#8A96A3]" title={detail}>{detail}</div>}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <span
            className="rounded-[3px] px-1.5 py-[1px] text-[10px] font-semibold uppercase tracking-wide"
            style={{ color: SEV_COLOR[ev.severity], background: `${SEV_COLOR[ev.severity]}1A` }}
          >
            {SEV_LABEL[ev.severity]}
          </span>
          <span title={fullTimestamp(ev.lastSeen)} className="font-mono text-[11px] text-[#5C6B7A]">
            {timeAgo(ev.lastSeen)}
          </span>
        </div>
      </div>
    );

    if (!href) return <div className="opacity-90">{content}</div>;
    return (
      <Link href={href} className="block hover:bg-[#F9FAFC]">
        {content}
      </Link>
    );
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <Cpu size={13} /> Monitoring
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">SNMP & Syslog Monitoring</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">
          Device, interface, and syslog events in one place. Click any event to jump to its device.
        </p>
      </div>

      <div className="mb-4 flex flex-col gap-2.5 sm:flex-row">
        <StatCard label="Critical" value={counts.critical} color={SEV_COLOR.critical} icon={<AlertTriangle size={16} />} />
        <StatCard label="Warning" value={counts.warning} color={SEV_COLOR.warning} icon={<AlertTriangle size={16} />} />
        <StatCard label="Unresolved" value={counts.unresolved} color="#1F2A37" icon={<Radar size={16} />} />
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative w-full max-w-xs">
          <Search size={13} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[#8A96A3]" />
          <EngInput value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search messages…" className="pl-7" />
        </div>
        <div className="w-[140px]">
          <EngSelect value={severity} onChange={(e) => setSeverity(e.target.value)}>
            <option value="">All severities</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="info">Info</option>
          </EngSelect>
        </div>
        <div className="w-[160px]">
          <EngSelect value={deviceId} onChange={(e) => setDeviceId(e.target.value)}>
            <option value="">All devices</option>
            {devices.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
          </EngSelect>
        </div>
        <div className="w-[150px]">
          <EngSelect value={groupId} onChange={(e) => setGroupId(e.target.value)}>
            <option value="">All groups</option>
            {groups.map((g) => <option key={g.id} value={String(g.id)}>{g.name}</option>)}
          </EngSelect>
        </div>
        <div className="w-[170px]">
          <EngSelect value={eventType} onChange={(e) => setEventType(e.target.value)}>
            <option value="">All event types</option>
            {EVENT_TYPES.map((t) => <option key={t} value={t}>{EVENT_TYPE_LABEL[t]}</option>)}
          </EngSelect>
        </div>
        <div className="w-[150px]">
          <EngSelect value={timeRange} onChange={(e) => setTimeRange(e.target.value)}>
            {TIME_RANGES.map((r) => <option key={r.key} value={r.key}>{r.label}</option>)}
          </EngSelect>
        </div>
        <div className="ml-auto flex rounded-[4px] border border-[#DCE1E8] bg-white p-0.5">
          <button
            onClick={() => setGrouped(false)}
            className={`inline-flex items-center gap-1.5 rounded-[3px] px-3 py-1 text-[12px] font-medium transition-colors duration-150 ${
              !grouped ? "bg-[#2E7BF6] text-white" : "text-[#5C6B7A] hover:bg-[#F4F6F9]"
            }`}
          >
            <List size={12} /> Flat
          </button>
          <button
            onClick={() => setGrouped(true)}
            className={`inline-flex items-center gap-1.5 rounded-[3px] px-3 py-1 text-[12px] font-medium transition-colors duration-150 ${
              grouped ? "bg-[#2E7BF6] text-white" : "text-[#5C6B7A] hover:bg-[#F4F6F9]"
            }`}
          >
            <Rows3 size={12} /> By device
          </button>
        </div>
      </div>

      {error && (
        <EngPanel className="mb-4">
          <div className="flex items-center justify-between py-2">
            <span className="flex items-center gap-1.5 text-[13px] text-[#C4362D]">
              <AlertTriangle size={13} /> {error}
            </span>
            <button
              onClick={loadEvents}
              className="inline-flex items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-3 py-1.5 text-[12px] font-medium text-[#1F2A37] hover:bg-[#F4F6F9]"
            >
              <RotateCw size={12} /> Retry
            </button>
          </div>
        </EngPanel>
      )}

      {loading && events.length === 0 ? (
        <EngPanel title="Loading…">
          <div className="flex flex-col divide-y divide-[#EEF1F4]">
            {Array.from({ length: 6 }).map((_, i) => <RowSkeleton key={i} />)}
          </div>
        </EngPanel>
      ) : !error && events.length === 0 ? (
        <EngPanel>
          <div className="py-10 text-center text-[13px] text-[#8A96A3]">No events in this range.</div>
        </EngPanel>
      ) : !grouped ? (
        <EngPanel title={`Events (${events.length}${hasMore ? "+" : ""})`}>
          <div className="-mx-3.5 -my-3.5 flex flex-col divide-y divide-[#EEF1F4]">
            {sortedEvents.map((ev) => <EventRow key={ev.id} ev={ev} />)}
          </div>
        </EngPanel>
      ) : (
        <div className="flex flex-col gap-3">
          {groupedSections!.map((section) => {
            const isCollapsed = collapsed.has(section.key);
            return (
              <div key={section.key} className="overflow-hidden rounded-[4px] border border-[#DCE1E8] bg-white">
                <button
                  onClick={() => toggleSection(section.key)}
                  className="flex w-full items-center gap-2 border-b border-[#DCE1E8] bg-[#F9FAFC] px-4 py-2.5 text-left hover:bg-[#F4F6F9]"
                >
                  {isCollapsed ? <ChevronRight size={13} className="text-[#5C6B7A]" /> : <ChevronDown size={13} className="text-[#5C6B7A]" />}
                  <span className="h-[7px] w-[7px] shrink-0 rounded-full" style={{ background: SEV_COLOR[section.worstSeverity] }} />
                  <h2 className="text-[13px] font-medium text-[#1F2A37]">{section.name}</h2>
                  <span className="text-[11px] text-[#8A96A3]">({section.items.length})</span>
                </button>
                {!isCollapsed && (
                  <div className="flex flex-col divide-y divide-[#EEF1F4]">
                    {section.items.map((ev) => <EventRow key={ev.id} ev={ev} />)}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {!loading && hasMore && events.length > 0 && (
        <div className="mt-3 flex justify-center">
          <button
            onClick={() => setLimit((l) => l + 100)}
            className="inline-flex items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-3.5 py-1.5 text-[12px] font-medium text-[#1F2A37] hover:bg-[#F4F6F9]"
          >
            Load more
          </button>
        </div>
      )}
    </main>
  );
}
