"use client";

// 40px NOC top bar: logo + system status on the left, open-alerts badge +
// pulse on the right. The alerts badge reads the global Zustand alert store
// (populated by the alert feed) so every surface shows one consistent count.
import { useEffect, useState } from "react";
import { useAlertStore } from "../lib/stores/alerts";
import { AlertBadge } from "./ui/primitives";

export function TopBar() {
  const alerts = useAlertStore((s) => s.alerts);
  const [now, setNow] = useState<Date>(() => new Date());

  useEffect(() => {
    const t = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(t);
  }, []);

  const openCount = alerts.filter(
    (a) => a.severity === "critical" || a.severity === "warning"
  ).length;

  const utc = now.toUTCString();

  return (
    <header className="flex h-10 shrink-0 items-center justify-between border-b border-[#21262D] bg-[#161B22] px-4">
      <div className="flex items-center gap-3">
        <span className="text-sm font-bold tracking-tight text-[#E6EDF3]">
          Routing<span className="text-[#3FB950]">NMS</span>
        </span>
        <span className="h-1.5 w-1.5 rounded-full bg-[#3FB950]" />
        <span className="text-xs text-[#8B949E]">All systems nominal</span>
        <span className="mono text-[#484F58]">{utc}</span>
      </div>
      <div className="flex items-center gap-2">
        <span className="label">Open alerts</span>
        <AlertBadge count={openCount} />
        <span className="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-[#3FB950] animate-pulse" />
      </div>
    </header>
  );
}
