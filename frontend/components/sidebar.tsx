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
  { name: "Topology", href: "/topology", icon: "☷" },
];

/**
 * Persistent left-hand NOC navigation, shared by every authenticated page
 * (see app/(noc)/layout.tsx). Owns session identity (username/logout) and a
 * lightweight backend-connectivity indicator so that state lives in one
 * place instead of being re-implemented per page.
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
    <aside className="flex h-screen w-64 shrink-0 flex-col border-r border-slate-800 bg-slate-900/60">
      <div className="border-b border-slate-800 px-5 py-5">
        <div className="text-lg font-bold tracking-tight text-white">RoutingNMS</div>
        <div className="mt-1 flex items-center gap-2 text-xs text-slate-400">
          <span className={`h-1.5 w-1.5 rounded-full ${connected ? "bg-emerald-400" : "bg-amber-400"}`} />
          {connected ? "Backend connected" : "Backend pending"}
        </div>
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        <div className="px-2 pb-2 text-[10px] font-semibold uppercase tracking-widest text-slate-600">Live NOC</div>
        {NAV.map((item) => {
          const active = pathname === item.href || pathname.startsWith(item.href + "/");
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors ${
                active
                  ? "bg-cyan-950/60 text-cyan-300 border border-cyan-800"
                  : "text-slate-300 border border-transparent hover:bg-slate-800/70 hover:text-white"
              }`}
            >
              <span className="w-4 text-center text-base leading-none" aria-hidden="true">{item.icon}</span>
              {item.name}
            </Link>
          );
        })}
      </nav>

      <div className="border-t border-slate-800 px-4 py-4">
        <div className="mb-3 truncate text-xs text-slate-400">
          {username ? <>Signed in as <span className="text-slate-200">{username}</span></> : " "}
        </div>
        <button
          onClick={logout}
          className="w-full rounded-lg border border-slate-700 px-3 py-2 text-xs text-slate-300 hover:bg-slate-800 hover:text-white"
        >
          Log out
        </button>
      </div>
    </aside>
  );
}
