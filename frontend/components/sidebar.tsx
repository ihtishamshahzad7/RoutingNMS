"use client";

import { useEffect, useMemo, useState } from "react";
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
  Cable,
  ScrollText,
  Zap,
  BookOpen,
  Wrench,
  CalendarClock,
  Tag as TagIcon,
  Gauge,
  Search,
  ChevronDown,
  LogOut,
  ShieldCheck,
} from "lucide-react";

type NavItem = { name: string; href: string; icon: React.ComponentType<{ size?: number; strokeWidth?: number }> };
type NavGroup = { label: string; items: NavItem[] };

/**
 * Accordion NOC navigation, modeled after Zabbix's collapsible sidebar:
 * a search box on top, then top-level sections that expand one at a time,
 * revealing their sub-items indented beneath. Every route that previously
 * had a sidebar entry still has one here — this is a reorganization of the
 * interaction model, not a feature removal.
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
      { name: "Topology Links", href: "/topology-links", icon: Cable },
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
  {
    label: "Account",
    items: [
      { name: "Settings", href: "/settings", icon: ShieldCheck },
    ],
  },
];

function groupContainsPath(group: NavGroup, pathname: string) {
  return group.items.some((item) => pathname === item.href || pathname.startsWith(item.href + "/"));
}

export default function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [connected, setConnected] = useState(false);
  const [query, setQuery] = useState("");
  const [openGroup, setOpenGroup] = useState<string | null>(() => {
    const active = NAV_GROUPS.find((g) => groupContainsPath(g, pathname));
    return active ? active.label : NAV_GROUPS[0]?.label ?? null;
  });

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

  const trimmedQuery = query.trim().toLowerCase();
  const isSearching = trimmedQuery.length > 0;

  const filteredGroups = useMemo(() => {
    if (!isSearching) return NAV_GROUPS;
    return NAV_GROUPS.map((group) => ({
      ...group,
      items: group.items.filter((item) => item.name.toLowerCase().includes(trimmedQuery)),
    })).filter((group) => group.items.length > 0);
  }, [isSearching, trimmedQuery]);

  function toggleGroup(label: string) {
    setOpenGroup((prev) => (prev === label ? null : label));
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

      <div className="px-3 pt-3">
        <div className="flex items-center gap-2 rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-1.5 text-[#8B949E] focus-within:border-[#58A6FF]">
          <Search size={14} strokeWidth={2} />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search"
            className="w-full bg-transparent text-sm text-[#E6EDF3] placeholder:text-[#484F58] focus:outline-none"
          />
        </div>
      </div>

      <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 py-3">
        {filteredGroups.map((group) => {
          const isActiveGroup = groupContainsPath(group, pathname);
          const isOpen = isSearching || openGroup === group.label;
          return (
            <div key={group.label}>
              <button
                type="button"
                onClick={() => toggleGroup(group.label)}
                className={`flex w-full items-center justify-between rounded-[5px] px-3 py-2 text-sm font-medium transition-colors duration-100 ${
                  isActiveGroup
                    ? "bg-[#1C2128] text-[#58A6FF]"
                    : "text-[#C9D1D9] hover:bg-[#1C2128] hover:text-[#E6EDF3]"
                }`}
              >
                <span>{group.label}</span>
                <ChevronDown
                  size={14}
                  strokeWidth={2}
                  className={`shrink-0 transition-transform duration-150 ${isOpen ? "rotate-180" : ""}`}
                />
              </button>

              {isOpen && (
                <div className="mt-0.5 space-y-0.5 border-l border-[#21262D] pl-3">
                  {group.items.map((item) => {
                    const active = pathname === item.href || pathname.startsWith(item.href + "/");
                    return (
                      <Link
                        key={item.href}
                        href={item.href}
                        className={`block rounded-[5px] px-3 py-1.5 text-[13px] transition-colors duration-100 ${
                          active
                            ? "bg-[#1C2128] font-semibold text-[#58A6FF]"
                            : "text-[#8B949E] hover:bg-[#1C2128] hover:text-[#E6EDF3]"
                        }`}
                      >
                        {item.name}
                      </Link>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
        {isSearching && filteredGroups.length === 0 && (
          <div className="px-3 py-2 text-xs text-[#484F58]">No matches</div>
        )}
      </nav>

      <div className="border-t border-[#21262D] px-3 py-3">
        <div className="mb-2 truncate px-1 text-xs text-[#484F58]">
          {username ? <>Signed in as <span className="text-[#8B949E]">{username}</span></> : " "}
        </div>
        <button
          onClick={logout}
          className="flex w-full items-center gap-2.5 rounded-[5px] px-3 py-1.5 text-[13px] text-[#8B949E] transition-colors duration-100 hover:bg-[#1C2128] hover:text-[#E6EDF3]"
        >
          <LogOut size={14} strokeWidth={2} />
          Log out
        </button>
      </div>
    </aside>
  );
}
