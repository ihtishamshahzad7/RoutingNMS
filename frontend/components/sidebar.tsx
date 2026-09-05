"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { apiFetch, ApiError } from "../lib/api";
import {
  LayoutDashboard,
  Server,
  Radar,
  Router,
  Users,
  MapPin,
  Wifi,
  AlertTriangle,
  Flame,
  Bell,
  Network,
  ScrollText,
  Zap,
  BookOpen,
  Wrench,
  CalendarClock,
  Tag as TagIcon,
  Gauge,
} from "lucide-react";

type NavItem = { name: string; href: string; icon: React.ComponentType<{ size?: number; strokeWidth?: number }> };
type NavGroup = { label: string; items: NavItem[] };

/**
 * Grouped NOC navigation, modeled after Uptime Kuma's short, scannable nav:
 * a handful of clear sections instead of one long flat list. Every route
 * that previously had a sidebar entry still has one here — this is a
 * reorganization, not a feature removal.
 */
const NAV_GROUPS: NavGroup[] = [
  {
    label: "Monitoring",
    items: [
      { name: "Dashboard", href: "/dashboard", icon: LayoutDashboard },
      { name: "Devices", href: "/devices", icon: Server },
      { name: "Reachability", href: "/reachability", icon: Radar },
    ],
  },
  {
    label: "Network",
    items: [
      { name: "OLTs", href: "/olts", icon: Router },
      { name: "Access Points", href: "/access-points", icon: Wifi },
      { name: "Topology", href: "/topology", icon: Network },
      { name: "Sites", href: "/sites", icon: MapPin },
      { name: "Customers", href: "/customers", icon: Users },
      { name: "Provisioning", href: "/provisioning", icon: Wrench },
    ],
  },
  {
    label: "Alerts & incidents",
    items: [
      { name: "Incidents", href: "/incidents", icon: AlertTriangle },
      { name: "Incident Hub", href: "/incident-hub", icon: Flame },
      { name: "Alert Rules", href: "/alert-rules", icon: Bell },
      { name: "Maintenance", href: "/maintenance", icon: CalendarClock },
    ],
  },
  {
    label: "Diagnostics",
    items: [
      { name: "Syslog", href: "/syslog", icon: ScrollText },
      { name: "SNMP Traps", href: "/traps", icon: Zap },
      { name: "MIBs", href: "/mibs", icon: BookOpen },
    ],
  },
  {
    label: "Organize",
    items: [
      { name: "Status Pages", href: "/status-pages", icon: Gauge },
      { name: "Tags", href: "/tags", icon: TagIcon },
    ],
  },
];

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

      <nav className="flex-1 space-y-4 overflow-y-auto px-3 py-4">
        {NAV_GROUPS.map((group) => (
          <div key={group.label}>
            <div className="label px-2 pb-1.5 text-[#484F58]">{group.label}</div>
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const active = pathname === item.href || pathname.startsWith(item.href + "/");
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`flex items-center gap-2.5 rounded-[5px] px-3 py-2 text-sm transition-colors duration-100 ${
                      active
                        ? "bg-[#1C2128] text-[#58A6FF] border border-[#30363D]"
                        : "text-[#8B949E] border border-transparent hover:bg-[#1C2128] hover:text-[#E6EDF3]"
                    }`}
                  >
                    <Icon size={16} strokeWidth={2} />
                    {item.name}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="border-t border-[#21262D] px-4 py-4">
        <div className="mb-3 truncate text-xs text-[#8B949E]">
          {username ? <>Signed in as <span className="text-[#E6EDF3]">{username}</span></> : " "}
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
