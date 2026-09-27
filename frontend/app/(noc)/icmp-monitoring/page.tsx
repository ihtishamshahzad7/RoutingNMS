"use client";

// Skeleton only (6-page rebuild, item 2) -- visual shell in the "Engineer
// Classic" design direction. Real functionality (devices grouped by
// Group with live status, per-device ICMP graph w/ 1h/24h/7d range) is
// wired in a later item; per the user's rule this page does not yet
// fetch or render live data.

import { Activity } from "lucide-react";
import { EngPanel } from "../../../components/ui/engineer";

export default function IcmpMonitoringPage() {
  return (
    <main className="mx-auto max-w-6xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <Activity size={13} /> Monitoring
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">ICMP Monitoring</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">
          Devices grouped by Group, with live status — click a device for its ICMP up/down and latency history.
        </p>
      </div>
      <EngPanel title="Devices by group">
        <div className="py-10 text-center text-[13px] text-[#8A96A3]">
          Skeleton only — live grouping, status and the per-device history graph land in a later build step.
        </div>
      </EngPanel>
    </main>
  );
}
