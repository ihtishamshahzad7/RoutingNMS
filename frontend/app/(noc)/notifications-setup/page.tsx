"use client";

// Skeleton only (6-page rebuild, item 5). Real channel configuration and
// working "test notification" buttons are wired in a later item.

import { BellRing } from "lucide-react";
import { EngPanel } from "../../../components/ui/engineer";

const CHANNELS = ["Discord", "Telegram", "Email", "Webhook"];

export default function NotificationsSetupPage() {
  return (
    <main className="mx-auto max-w-6xl px-6 py-6">
      <div className="mb-4">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-[#2E7BF6]">
          <BellRing size={13} /> Alerts &amp; incidents
        </div>
        <h1 className="mt-1 text-xl font-medium text-[#1F2A37]">Notifications Setup</h1>
        <p className="mt-1 text-[13px] text-[#5C6B7A]">
          Configure Discord, Telegram, Email and generic Webhook channels, with a working test notification per channel.
        </p>
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        {CHANNELS.map((c) => (
          <EngPanel key={c} title={c}>
            <div className="py-6 text-center text-[13px] text-[#8A96A3]">Skeleton only — configuration and test-send land in a later build step.</div>
          </EngPanel>
        ))}
      </div>
    </main>
  );
}
