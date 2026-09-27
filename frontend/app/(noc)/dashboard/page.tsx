"use client";

// Kuma-style dashboard: a left column listing every monitor (device) with a
// live heartbeat bar, uptime %, and status badge -- replaces the earlier
// customizable widget-grid dashboard per the Kuma core-parity freeze
// (2026-09-27), item 1. Items 2-8 of that freeze are not started yet; this
// page intentionally does nothing beyond item 1's own scope (no maintenance/
// tags/notification wiring here). The prior widget-grid page is kept as
// page.tsx.pre-kuma-freeze.bak for reference, not wired into any route.

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "../../../lib/api";
import { StatusPill } from "../../../components/ui/status-pill";
import { MonitorList, type MonitorRow } from "./MonitorList";
import type { Beat } from "./HeartbeatBar";

const ORG = "tenant-1";

type Device = { id: string; name: string; address: string; deviceType: string; enabled: boolean };
type DeviceUptime = { deviceId: string; status: "up" | "down" | "warning" | "unknown"; uptime24h?: number | null; uptime7d?: number | null };
type UptimeSummaryResponse = { devices: DeviceUptime[]; up: number; down: number; warning: number; unknown: number; total: number };
type ProbeResult = { probedAt: string; lossPct: number; isReachable: boolean };
type LiveResponse = { live: { reachable: boolean }; history: ProbeResult[] };

export default function DashboardPage() {
  const router = useRouter();
  const [devices, setDevices] = useState<Device[]>([]);
  const [uptime, setUptime] = useState<UptimeSummaryResponse | null>(null);
  const [beatsById, setBeatsById] = useState<Record<string, Beat[]>>({});
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let active = true;

    const load = async () => {
      try {
        const list = await apiFetch<Device[]>(`/devices?organizationId=${ORG}`);
        if (!active) return;
        setDevices(list);
        setSelectedId((prev) => prev ?? list[0]?.id);
      } catch (e) {
        if (active && e instanceof ApiError && e.status === 401) router.replace("/");
      }
      try {
        const u = await apiFetch<UptimeSummaryResponse>(`/devices/uptime-summary?organizationId=${ORG}`);
        if (active) setUptime(u);
      } catch {
        // Uptime rollup is a nice-to-have on top of per-device heartbeats below.
      }
      setLoading(false);
    };

    load();
    const t = window.setInterval(load, 15000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router]);

  // Per-device heartbeat history (last ~50-60 probes), matching Kuma's
  // monitor-list bar. Fetched in parallel, same pattern already used
  // elsewhere in this app for per-item fan-out (e.g. the OLT alerts fetch).
  useEffect(() => {
    let active = true;
    const load = async () => {
      const entries = await Promise.all(
        devices.map(async (d) => {
          try {
            const r = await apiFetch<LiveResponse>(`/ping/${d.id}/live`);
            const beats: Beat[] = r.history.map((h) => ({ reachable: h.isReachable, lossPct: h.lossPct, probedAt: h.probedAt }));
            return [d.id, beats] as const;
          } catch {
            return [d.id, []] as const;
          }
        })
      );
      if (active) setBeatsById(Object.fromEntries(entries));
    };
    if (devices.length) load();
    const t = window.setInterval(() => devices.length && load(), 15000);
    return () => {
      active = false;
      window.clearInterval(t);
    };
  }, [devices]);

  const uptimeById = useMemo(() => {
    const map: Record<string, DeviceUptime> = {};
    for (const d of uptime?.devices ?? []) map[d.deviceId] = d;
    return map;
  }, [uptime]);

  const monitors: MonitorRow[] = devices.map((d) => {
    const u = uptimeById[d.id];
    return {
      id: d.id,
      name: d.name,
      address: d.address,
      status: u?.status ?? "unknown",
      uptime24h: u?.uptime24h,
      beats: beatsById[d.id] ?? [],
    };
  });

  const selected = devices.find((d) => d.id === selectedId);
  const selectedUptime = selectedId ? uptimeById[selectedId] : undefined;

  return (
    <main className="mx-auto flex max-w-[1400px] gap-5 px-6 py-6">
      <div className="w-[360px] shrink-0">
        <div className="mb-3 flex items-center justify-between">
          <h1 className="text-[15px] font-bold text-[#E6EDF3]">Monitors</h1>
          {uptime && (
            <span className="text-[11px] text-[#8B949E]">
              <span className="text-[#3FB950]">{uptime.up}</span> up · <span className="text-[#F78166]">{uptime.down}</span> down
              {uptime.warning ? (
                <>
                  {" "}
                  · <span className="text-[#D29922]">{uptime.warning}</span> warn
                </>
              ) : null}
            </span>
          )}
        </div>
        {loading ? (
          <div className="px-3 py-6 text-center text-xs text-[#8B949E]">Loading monitors…</div>
        ) : (
          <MonitorList monitors={monitors} selectedId={selectedId} onSelect={setSelectedId} />
        )}
      </div>

      <div className="min-w-0 flex-1 rounded-[8px] border border-[#21262D] bg-[#161B22] p-5">
        {selected ? (
          <div>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-[16px] font-bold text-[#E6EDF3]">{selected.name}</div>
                <div className="mt-0.5 text-[12px] text-[#8B949E]">{selected.address}</div>
              </div>
              <StatusPill
                status={selectedUptime?.status ?? "unknown"}
                label={selectedUptime?.status ? selectedUptime.status[0].toUpperCase() + selectedUptime.status.slice(1) : "Pending"}
                pulse
              />
            </div>
            <div className="mt-4 text-[12px] text-[#8B949E]">
              Uptime (24h): <span className="font-mono text-[#E6EDF3]">{selectedUptime?.uptime24h != null ? `${selectedUptime.uptime24h.toFixed(2)}%` : "—"}</span>
            </div>
            <p className="mt-6 text-xs text-[#484F58]">
              Full monitor detail (response-time chart, 24h/30d/1y uptime, event log) lands with item 3 of the Kuma
              parity freeze — not built yet.
            </p>
          </div>
        ) : (
          <div className="flex h-full items-center justify-center text-xs text-[#8B949E]">Select a monitor</div>
        )}
      </div>
    </main>
  );
}
