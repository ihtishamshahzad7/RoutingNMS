"use client";

// Connectivity Monitoring (6-page rebuild, item 2, extended by 2.2):
// devices grouped by their assigned Group, each row showing ICMP status
// AND Port/Service check status side by side -- a device can have either,
// both, or neither configured. Click through to the per-device history
// graphs (app/(noc)/icmp-monitoring/[id]/page.tsx). Kept at the same route
// (/icmp-monitoring) it launched on; only the on-page title/labels changed
// to "Connectivity Monitoring" to reflect that it now covers both checks.

import { useEffect, useState } from "react";
import Link from "next/link";
import { Activity } from "lucide-react";
import { apiFetch } from "../../../lib/api";
import { EngPanel } from "../../../components/ui/engineer";
import { groupSections, type DeviceGroup, type GroupMember } from "../../../lib/device-groups";

const ORG = "tenant-1";

type Device = { id: string; name: string; address: string; icmpEnabled: boolean; portCheckEnabled: boolean };
type DeviceUptime = { deviceId: string; status: "up" | "down" | "warning" | "unknown"; uptime24h?: number | null };
type UptimeSummaryResponse = { devices: DeviceUptime[] };
type PortCheckRow = { deviceId: string; protocol: string; port: number; reachable: boolean; latencyMs: number };
type PortCheckSummaryResponse = { devices: PortCheckRow[] };

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

export default function ConnectivityMonitoringPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [memberOf, setMemberOf] = useState<Record<string, number>>({});
  const [uptimeById, setUptimeById] = useState<Record<string, DeviceUptime>>({});
  const [portById, setPortById] = useState<Record<string, PortCheckRow>>({});
  const [loading, setLoading] = useState(true);

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
    } catch {
      // best-effort -- an empty list still renders a usable (if bare) page
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); const t = window.setInterval(load, 15000); return () => window.clearInterval(t); }, []);

  const sections = groupSections(devices, groups, memberOf);

  return (
    <main className="mx-auto max-w-5xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <Activity size={13} /> Monitoring
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">Connectivity Monitoring</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">Devices grouped by Group, with ICMP and Port/Service check status side by side. Click a device for its history.</p>
      </div>

      {loading ? (
        <EngPanel><div className="py-10 text-center text-[13px] text-[#8A96A3]">Loading…</div></EngPanel>
      ) : (
        <div className="flex flex-col gap-4">
          {sections.map((section) => (
            <EngPanel key={section.key} title={`${section.name} (${section.items.length})`}>
              <div className="flex flex-col divide-y divide-[#EEF1F4]">
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
            </EngPanel>
          ))}
          {devices.length === 0 && <EngPanel><div className="py-10 text-center text-[13px] text-[#8A96A3]">No devices registered yet.</div></EngPanel>}
        </div>
      )}
    </main>
  );
}
