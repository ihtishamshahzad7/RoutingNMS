"use client";

// Kuma-style monitor list: a left column of every device, each row showing
// name, status badge, uptime %, and a live heartbeat bar of its last ~50
// checks -- mirrors Uptime Kuma's classic dashboard monitor list.
//
// Item 1 of the Kuma core-parity freeze (2026-09-27).

import Link from "next/link";
import { StatusPill } from "../../../components/ui/status-pill";
import { HeartbeatBar, type Beat } from "./HeartbeatBar";

export type MonitorRow = {
  id: string;
  name: string;
  address: string;
  status: "up" | "down" | "warning" | "unknown";
  uptime24h?: number | null;
  beats: Beat[];
};

function StatusLabel({ status }: { status: MonitorRow["status"] }) {
  const label = status === "up" ? "Up" : status === "down" ? "Down" : status === "warning" ? "Warning" : "Pending";
  return <StatusPill status={status} label={label} pulse={status === "up"} />;
}

export function MonitorList({ monitors, selectedId, onSelect }: { monitors: MonitorRow[]; selectedId?: string; onSelect?: (id: string) => void }) {
  return (
    <div className="flex flex-col divide-y divide-[#21262D] rounded-[8px] border border-[#21262D] bg-[#161B22]">
      {monitors.map((m) => {
        const active = m.id === selectedId;
        return (
          <button
            key={m.id}
            type="button"
            onClick={() => onSelect?.(m.id)}
            className={`flex w-full flex-col gap-1.5 px-3 py-2.5 text-left transition-colors duration-100 ${
              active ? "bg-[#1C2128]" : "hover:bg-[#1C2128]"
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <span className="truncate text-[13px] font-semibold text-[#E6EDF3]">{m.name}</span>
              </div>
              <StatusLabel status={m.status} />
            </div>
            <div className="flex items-center justify-between gap-2 text-[11px] text-[#8B949E]">
              <span className="truncate">{m.address}</span>
              <span className="shrink-0 font-mono">{m.uptime24h != null ? `${m.uptime24h.toFixed(1)}%` : "—"}</span>
            </div>
            <HeartbeatBar beats={m.beats} height={22} />
          </button>
        );
      })}
      {monitors.length === 0 && (
        <div className="px-3 py-6 text-center text-xs text-[#8B949E]">
          No monitors yet. <Link href="/devices" className="text-[#58A6FF]">Add one →</Link>
        </div>
      )}
    </div>
  );
}
