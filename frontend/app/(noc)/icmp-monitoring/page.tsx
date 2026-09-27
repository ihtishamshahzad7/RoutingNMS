"use client";

// ICMP Monitoring (6-page rebuild, item 2): devices grouped by their
// assigned Group, each row showing live status -- click through to the
// per-device history graph (app/(noc)/icmp-monitoring/[id]/page.tsx).
// Reuses the same backend the Devices/Dashboard pages already use: no new
// endpoints needed for the list view.

import { useEffect, useState } from "react";
import Link from "next/link";
import { Activity } from "lucide-react";
import { apiFetch } from "../../../lib/api";
import { EngPanel } from "../../../components/ui/engineer";
import { groupSections, type DeviceGroup, type GroupMember } from "../../../lib/device-groups";

const ORG = "tenant-1";

type Device = { id: string; name: string; address: string };
type DeviceUptime = { deviceId: string; status: "up" | "down" | "warning" | "unknown"; uptime24h?: number | null };
type UptimeSummaryResponse = { devices: DeviceUptime[] };

const STATUS_DOT: Record<string, string> = {
  up: "#1E8E5A",
  down: "#C4362D",
  warning: "#C77700",
  unknown: "#8A96A3",
};

export default function IcmpMonitoringPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [groups, setGroups] = useState<DeviceGroup[]>([]);
  const [memberOf, setMemberOf] = useState<Record<string, number>>({});
  const [uptimeById, setUptimeById] = useState<Record<string, DeviceUptime>>({});
  const [loading, setLoading] = useState(true);

  async function load() {
    setLoading(true);
    try {
      const [ds, gs, ms, summary] = await Promise.all([
        apiFetch<Device[]>(`/devices?organizationId=${ORG}`),
        apiFetch<DeviceGroup[]>(`/device-groups?tenantId=${ORG}`),
        apiFetch<GroupMember[]>("/device-groups/members"),
        apiFetch<UptimeSummaryResponse>(`/devices/uptime-summary?organizationId=${ORG}`),
      ]);
      setDevices(ds);
      setGroups(gs);
      const map: Record<string, number> = {};
      ms.filter((m) => m.subjectType === "device").forEach((m) => { map[m.subjectId] = m.groupId; });
      setMemberOf(map);
      const u: Record<string, DeviceUptime> = {};
      summary.devices.forEach((d) => { u[d.deviceId] = d; });
      setUptimeById(u);
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
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">ICMP Monitoring</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">Devices grouped by Group, with live status. Click a device for its ICMP history.</p>
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
                  return (
                    <Link key={d.id} href={`/icmp-monitoring/${d.id}`} className="flex items-center justify-between gap-3 px-1 py-2 hover:bg-[#F9FAFC]">
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="h-2 w-2 shrink-0 rounded-full" style={{ background: STATUS_DOT[u?.status ?? "unknown"] }} />
                        <span className="truncate text-[13px] font-medium text-[#1F2A37]">{d.name}</span>
                        <span className="truncate font-mono text-[11px] text-[#8A96A3]">{d.address}</span>
                      </div>
                      <span className="shrink-0 font-mono text-[11px] text-[#5C6B7A]">
                        {u?.uptime24h != null ? `${u.uptime24h.toFixed(1)}% (24h)` : "—"}
                      </span>
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
