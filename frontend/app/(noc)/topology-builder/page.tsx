"use client";

// Skeleton only (6-page rebuild, item 6). The real drag-and-drop canvas,
// per-port link drawing and live link status are wired in a later item.

import { Waypoints } from "lucide-react";
import { EngPanel } from "../../../components/ui/engineer";

export default function TopologyBuilderPage() {
  return (
    <main className="mx-auto max-w-6xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <Waypoints size={13} /> Network
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">Topology Builder</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">
          A visual canvas — drag devices on, draw links between specific ports, see live up/down link color.
        </p>
      </div>
      <EngPanel title="Canvas">
        <div className="flex h-[420px] items-center justify-center rounded-[4px] border border-dashed border-[#DCE1E8] text-[13px] text-[#8A96A3]">
          Skeleton only — the drag-and-drop canvas lands in a later build step.
        </div>
      </EngPanel>
    </main>
  );
}
