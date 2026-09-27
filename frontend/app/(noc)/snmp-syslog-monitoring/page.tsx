"use client";

// Skeleton only (6-page rebuild, item 3). Real functionality (up/down,
// critical alerts, high CPU, interface loop detection, per-port history)
// is wired in a later item.

import { Cpu } from "lucide-react";
import { EngPanel } from "../../../components/ui/engineer";

export default function SnmpSyslogMonitoringPage() {
  return (
    <main className="mx-auto max-w-6xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <Cpu size={13} /> Monitoring
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">SNMP & Syslog Monitoring</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">
          Devices and events: up/down status, critical alerts, high CPU, interface loop detection — click a device for per-port history.
        </p>
      </div>
      <EngPanel title="Devices & events">
        <div className="py-10 text-center text-[13px] text-[#8A96A3]">
          Skeleton only — live SNMP/syslog events and the per-port detail view land in a later build step.
        </div>
      </EngPanel>
    </main>
  );
}
