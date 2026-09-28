"use client";

// Connectivity Monitoring (6-page rebuild, item 2, extended by 2.2, 2.5):
// devices grouped by their assigned Group, each row showing ICMP status
// AND Port/Service check status side by side -- a device can have either,
// both, or neither configured. Click through to the per-device history
// graphs (app/(noc)/icmp-monitoring/[id]/page.tsx). Kept at the same route
// (/icmp-monitoring) it launched on; only the on-page title/labels changed
// to "Connectivity Monitoring" to reflect that it now covers both checks.
//
// 2.5: UI polish -- sticky collapsible group headers, a status filter
// (All/Up/Down) and a name/IP search box, skeleton loading instead of
// "Loading…" text, and an inline error + Retry instead of a silently
// stale/blank page on fetch failure.

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Activity, ChevronDown, ChevronRight, FolderTree, RotateCw, Search } from "lucide-react";
import { apiFetch, ApiError } from "../../../lib/api";
import { EngPanel, EngInput } from "../../../components/ui/engineer";
import { groupSections, type DeviceGroup, type GroupMember } from "../../../lib/device-groups";

const ORG = "tenant-1";

type Device = { id: string; name: string; address: string; icmpEnabled: boolean; portCheckEnabled: boolean };
type DeviceUptime = { deviceId: string; status: "up" | "down" | "warning" | "unknown"; uptime24h?: number | null };
type UptimeSummaryResponse = { devices: DeviceUptime[] };
type PortCheckRow = { deviceId: string; protocol: string; port: number; reachable: boolean; latencyMs: number };
type PortCheckSummaryResponse = { devices: PortCheckRow[] };
type StatusFilter = "all" | "up" | "down";

const STATUS_DOT: Record<string, string> = {
  up: "#1E8E5A",
  down: "#C4362D",
  warning: "#C77700",
  unknown: "#8A96A3",
};

function StatusChip({ label, color, title }: { label: string; color: string; title: string }) {
  return (
    <span title={title} className="inline-flex items-center gap-1 rounded-[3px] border border-[#EEF1F4] bg-[#F9FAFC] px-1.5 py-[1px]">
      <span className="h-[7px] w-[7px] shrink-0 rounded-full" style={{ background: color }} />
      <span className="text-[10px] font-medium uppercase tracking-wide text-[#5C6B7A]">{label}</span>
    </span>
  );
}

/** A device's combined status for the All/Up/Down filter: down if either
 * configured check is down, up if every configured check is up, and
 * excluded from the Up/Down filters entirely when nothing is configured. */
function combinedStatus(d: Device, u: DeviceUptime | undefined, pc: PortCheckRow | undefined): "up" | "down" | null {
  if (!d.icmpEnabled && !d.portCheckEnabled) return null;
  let sawDown = false;
  let sawKnown = false;
  if (d.icmpEnabled && u) {
    sawKnown = true;
    if (u.status === "down") sawDown = true;
  }
  if (d.portCheckEnabled && pc) {
    sawKnown = true;
    if (!pc.reachable) sawDown = true;
  }
  if (!sawKnown) return null;
  return sawDown ? "down" : "up";
}

function RowSkeleton() {
  return (
    <div className="flex animate-pulse items-center justify-between gap-3 px-1 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <div className="h-[13px] w-28 rounded-[3px] bg-[#EEF1F4]" />
        <div className="h-[11px] w-24 rounded-[3px] bg-[#EEF1F4]" />
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <div className="h-[17px] w-14 rounded-[3px] bg-[#EEF1F4]" />
        <div className="h-[17px] w-14 rounded-[3px] bg-[#EEF1F4]" />
        <div className="h-[11px] w-16 rounded-[3px] bg-[#EEF1F4]" />
      </div>
    </div>
  );
}

export default function ConnectivityMonitoringPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [memberOf, setMemberOf] = useState<Record<string, number>>({});
  const [uptimeById, setUptimeById] = useState<Record<string, DeviceUptime>>({});
  const [portById, setPortById] = useState<Record<string, PortCheckRow>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  async function load() {
    setLoading(true);
    try {
      const [ds, gs, ms, summary, portSummary] = await Promise.all([
        apiFetch<Device[]>(`/devices?organizationId=${ORG}`),
        apiFetch<DeviceGroup[]>(`/device-groups?tenantId=${ORG}`),
        apiFetch<GroupMember[]>("/device-groups/members"),
        apiFetch<UptimeSummaryResponse>(`/devices/uptime-summary?organizationId=${ORG}`),
        apiFetch<PortCheckSummaryResponse>(`/port-check/summary?organizationId=${ORG}`),
      ]);
      setDevices(ds);
      setGroups(gs);
      const map: Record<string, number> = {};
      ms.filter((m) => m.subjectType === "device").forEach((m) => { map[m.subjectId] = m.groupId; });
      setMemberOf(map);
      const u: Record<string, DeviceUptime> = {};
      summary.devices.forEach((d) => { u[d.deviceId] = d; });
      setUptimeById(u);
      const p: Record<string, PortCheckRow> = {};
      portSummary.devices.forEach((d) => { p[d.deviceId] = d; });
      setPortById(p);
      setError("");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Unable to reach the monitoring API.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); const t = window.setInterval(load, 15000); return () => window.clearInterval(t); }, []);

  const filteredDevices = useMemo(() => {
    const q = search.trim().toLowerCase();
    return devices.filter((d) => {
      if (q && !d.name.toLowerCase().includes(q) && !d.address.toLowerCase().includes(q)) return false;
      if (statusFilter !== "all") {
        const status = combinedStatus(d, uptimeById[d.id], portById[d.id]);
        if (status !== statusFilter) return false;
      }
      return true;
    });
  }, [devices, search, statusFilter, uptimeById, portById]);

  const sections = groupSections(filteredDevices, groups, memberOf);

  function toggleSection(key: string) {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key); else next.add(key);
      return next;
    });
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <Activity size={13} /> Monitoring
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">Connectivity Monitoring</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">Devices grouped by Group, with ICMP and Port/Service check status side by side. Click a device for its history.</p>
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative w-full max-w-xs">
          <Search size={13} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-[#8A96A3]" />
          <EngInput
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name or IP…"
            className="pl-7"
          />
        </div>
        <div className="flex rounded-[4px] border border-[#DCE1E8] bg-white p-0.5">
          {([
            { key: "all", label: "All" },
            { key: "up", label: "Up" },
            { key: "down", label: "Down" },
          ] as { key: StatusFilter; label: string }[]).map((f) => (
            <button
              key={f.key}
              onClick={() => setStatusFilter(f.key)}
              className={`rounded-[3px] px-3 py-1 text-[12px] font-medium transition-colors duration-150 ${
                statusFilter === f.key ? "bg-[#2E7BF6] text-white" : "text-[#5C6B7A] hover:bg-[#F4F6F9]"
              }`}
            >
              {f.label}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <EngPanel className="mb-4">
          <div className="flex items-center justify-between py-2">
            <span className="text-[13px] text-[#C4362D]">{error}</span>
            <button
              onClick={load}
              className="inline-flex items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-3 py-1.5 text-[12px] font-medium text-[#1F2A37] hover:bg-[#F4F6F9]"
            >
              <RotateCw size={12} /> Retry
            </button>
          </div>
        </EngPanel>
      )}

      {loading && devices.length === 0 ? (
        <EngPanel title="Loading…">
          <div className="flex flex-col divide-y divide-[#EEF1F4]">
            {Array.from({ length: 4 }).map((_, i) => <RowSkeleton key={i} />)}
          </div>
        </EngPanel>
      ) : (
        <div className="flex flex-col gap-4">
          {sections.map((section) => {
            const isCollapsed = collapsed.has(section.key);
            return (
              <div key={section.key} className="overflow-hidden rounded-[4px] border border-[#DCE1E8] bg-white">
                <button
                  onClick={() => toggleSection(section.key)}
                  className="sticky top-0 z-10 flex w-full items-center gap-2 rounded-t-[4px] border-b border-[#DCE1E8] bg-[#F9FAFC] px-4 py-2.5 text-left hover:bg-[#F4F6F9]"
                >
                  {isCollapsed ? <ChevronRight size={13} className="text-[#5C6B7A]" /> : <ChevronDown size={13} className="text-[#5C6B7A]" />}
                  <FolderTree size={13} className="text-[#5C6B7A]" />
                  <h2 className="text-[13px] font-medium text-[#1F2A37]">{section.name}</h2>
                  <span className="text-[11px] text-[#8A96A3]">({section.items.length})</span>
                </button>
                {!isCollapsed && (
                  <div className="flex flex-col divide-y divide-[#EEF1F4] p-3.5">
                    {section.items.map((d) => {
                      const u = uptimeById[d.id];
                      const pc = portById[d.id];
                      return (
                        <Link key={d.id} href={`/icmp-monitoring/${d.id}`} className="flex items-center justify-between gap-3 px-1 py-2 hover:bg-[#F9FAFC]">
                          <div className="flex min-w-0 items-center gap-2">
                            <span className="truncate text-[13px] font-medium text-[#1F2A37]">{d.name}</span>
                            <span className="truncate font-mono text-[11px] text-[#8A96A3]">{d.address}</span>
                          </div>
                          <div className="flex shrink-0 items-center gap-2">
                            {d.icmpEnabled && (
                              <StatusChip label="ICMP" color={STATUS_DOT[u?.status ?? "unknown"]} title={`ICMP: ${u?.status ?? "unknown"}`} />
                            )}
                            {d.portCheckEnabled && (
                              <StatusChip
                                label={pc ? pc.protocol.toUpperCase() : "PORT"}
                                color={pc ? (pc.reachable ? STATUS_DOT.up : STATUS_DOT.down) : STATUS_DOT.unknown}
                                title={pc ? `Port/Service (${pc.protocol.toUpperCase()} :${pc.port}): ${pc.reachable ? "up" : "down"}` : "Port/Service: not yet checked"}
                              />
                            )}
                            {!d.icmpEnabled && !d.portCheckEnabled && <span className="text-[11px] text-[#8A96A3]">No checks configured</span>}
                            <span className="font-mono text-[11px] text-[#5C6B7A]">
                              {u?.uptime24h != null ? `${u.uptime24h.toFixed(1)}% (24h)` : "—"}
                            </span>
                          </div>
                        </Link>
                      );
                    })}
                    {section.items.length === 0 && <div className="px-1 py-4 text-[12px] text-[#8A96A3]">No devices in this group.</div>}
                  </div>
                )}
              </div>
            );
          })}
          {!loading && devices.length === 0 && !error && (
            <EngPanel><div className="py-10 text-center text-[13px] text-[#8A96A3]">No devices registered yet.</div></EngPanel>
          )}
          {!loading && devices.length > 0 && filteredDevices.length === 0 && (
            <EngPanel><div className="py-10 text-center text-[13px] text-[#8A96A3]">No devices match this search/filter.</div></EngPanel>
          )}
        </div>
      )}
    </main>
  );
}
