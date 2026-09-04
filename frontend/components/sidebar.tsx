"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { apiFetch, ApiError } from "../lib/api";

const NAV = [
  { name: "Dashboard", href: "/dashboard", icon: "▦" },
  { name: "Devices", href: "/devices", icon: "▣" },
  { name: "OLTs", href: "/olts", icon: "◈" },
  { name: "Incidents", href: "/incidents", icon: "⚠" },
  { name: "Incident Hub", href: "/incident-hub", icon: "◉" },
  { name: "Alert Rules", href: "/alert-rules", icon: "⚑" },
  { name: "Topology", href: "/topology", icon: "☷" },
  { name: "Syslog", href: "/syslog", icon: "☰" },
  { name: "SNMP Traps", href: "/traps", icon: "⚡" },
  { name: "MIBs", href: "/mibs", icon: "▤" },
  { name: "Sites", href: "/sites", icon: "⌖" },
  { name: "Access Points", href: "/access-points", icon: "◬" },
  { name: "Customers", href: "/customers", icon: "◉" },
  { name: "Provisioning", href: "/provisioning", icon: "⚙" },
  { name: "Status Pages", href: "/status-pages", icon: "◔" },
  { name: "Maintenance", href: "/maintenance", icon: "⛭" },
];

/**
 * Persistent left-hand NOC navigation, shared by every authenticated page
 * (see app/(noc)/layout.tsx). Owns session identity (username/logout) and a
 * lightweight backend-connectivity indicator. Restyled to the GitHub-dark NOC
 * palette; all behaviour is unchanged from the original.
 */
export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let active = true;
    apiFetch<{ username: string }>("/auth/me")
      .then((me) => { if (active) setUsername(me.username); })
      .catch((err) => { if (active && err instanceof ApiError && err.status === 401) router.replace("/"); });
    return () => { active = false; };
  }, [router]);

  useEffect(() => {
    let active = true;
    const check = async () => {
      try {
        const res = await fetch("/api/v1/health", { cache: "no-store" });
        if (active) setConnected(res.ok);
      } catch {
        if (active) setConnected(false);
      }
    };
    check();
    const timer = window.setInterval(check, 15000);
    return () => { active = false; window.clearInterval(timer); };
  }, []);

  async function logout() {
    try { await apiFetch("/auth/logout", { method: "POST" }); }
    finally { router.replace("/"); router.refresh(); }
  }

  return (
    <aside className="flex h-screen w-64 shrink-0 flex-col border-r border-[#21262D] bg-[#161B22]">
      <div className="border-b border-[#21262D] px-5 py-5">
        <div className="text-lg font-bold tracking-tight text-[#E6EDF3]">
          Routing<span className="text-[#3FB950]">NMS</span>
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs text-[#8B949E]">
          <span className={`h-1.5 w-1.5 rounded-full ${connected ? "bg-[#3FB950]" : "bg-[#D29922]"}`} />
          {connected ? "Backend connected" : "Backend pending"}
        </div>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        <div className="label px-2 pb-2 text-[#484F58]">Live NOC</div>
        {NAV.map((item) => {
          const active = pathname === item.href || pathname.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 rounded-[5px] px-3 py-2 text-sm transition-colors duration-100 ${
                active
                  ? "bg-[#1C2128] text-[#58A6FF] border border-[#30363D]"
                  : "text-[#8B949E] border border-transparent hover:bg-[#1C2128] hover:text-[#E6EDF3]"
              }`}
            >
              <span className="w-4 text-center text-base leading-none" aria-hidden="true">{item.icon}</span>
              {item.name}
            </Link>
          );
        })}
      </nav>

      <div className="border-t border-[#21262D] px-4 py-4">
        <div className="mb-3 truncate text-xs text-[#8B949E]">
          {username ? <>Signed in as <span className="text-[#E6EDF3]">{username}</span></> : "\u00A0"}
        </div>
        <button
          onClick={logout}
          className="w-full rounded-[5px] border border-[#30363D] bg-[#21262D] px-3 py-2 text-xs text-[#E6EDF3] transition-colors duration-100 hover:bg-[#1C2128]"
        >
          Log out
        </button>
      </div>
    </aside>
  );
}
