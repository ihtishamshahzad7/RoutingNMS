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
  BellRing,
  Network,
  Cable,
  ScrollText,
  Zap,
  Cpu,
  Waypoints,
  Activity,
  BookOpen,
  Wrench,
  CalendarClock,
  Tag as TagIcon,
  FolderTree,
  Gauge,
  Search,
  ScanSearch,
  ChevronDown,
  LogOut,
  ShieldCheck,
  Palette,
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
      { name: "Connectivity Monitoring", href: "/icmp-monitoring", icon: Activity },
      { name: "SNMP & Syslog Monitoring", href: "/snmp-syslog-monitoring", icon: Cpu },
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
      { name: "Topology Builder", href: "/topology-builder", icon: Waypoints },
      { name: "Workspace Topology", href: "/workspace-topology", icon: FolderTree },
      { name: "Sites", href: "/sites", icon: MapPin },
      { name: "Customers", href: "/customers", icon: Users },
      { name: "Provisioning", href: "/provisioning", icon: Wrench },
      { name: "Auto-Discovery", href: "/auto-discovery", icon: ScanSearch },
    ],
  },
  {
    label: "Alerts & incidents",
    items: [
      { name: "Incidents", href: "/incidents", icon: AlertTriangle },
      { name: "Incident Hub", href: "/incident-hub", icon: Flame },
      { name: "Alert Rules", href: "/alert-rules", icon: Bell },
      { name: "Notifications Setup", href: "/notifications-setup", icon: BellRing },
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
      { name: "Device Groups", href: "/device-groups", icon: FolderTree },
    ],
  },
  {
    label: "Account",
    items: [
      { name: "Settings", href: "/settings", icon: ShieldCheck },
      { name: "Style Guide", href: "/style-guide", icon: Palette },
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
    <aside className="flex h-screen w-64 shrink-0 flex-col bg-[#1B2A41]">
      <div className="px-5 py-5">
        <div className="text-lg font-medium tracking-tight text-white">
          Routing<span className="text-[#2E7BF6]">NMS</span>
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs text-[#C4CDD9]">
          <span className={`h-1.5 w-1.5 rounded-full ${connected ? "bg-[#1E8E5A]" : "bg-[#C77700]"}`} />
          {connected ? "Backend connected" : "Backend pending"}
        </div>
      </div>

      <div className="px-3 pt-1 pb-3">
        <div className="flex items-center gap-2 rounded-[4px] border border-white/10 bg-[#152337] px-3 py-1.5 text-[#C4CDD9] focus-within:border-[#2E7BF6]">
          <Search size={14} strokeWidth={2} />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search"
            className="w-full bg-transparent text-[13px] text-white placeholder:text-[#7E8CA0] focus:outline-none"
          />
        </div>
      </div>

      <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 py-1">
        {filteredGroups.map((group) => {
          const isActiveGroup = groupContainsPath(group, pathname);
          const isOpen = isSearching || openGroup === group.label;
          return (
            <div key={group.label}>
              <button
                type="button"
                onClick={() => toggleGroup(group.label)}
                className={`flex w-full items-center justify-between rounded-[4px] px-3 py-2 text-[13px] font-medium transition-colors duration-150 ${
                  isActiveGroup ? "text-white" : "text-[#C4CDD9] hover:bg-[#26374F] hover:text-white"
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
                <div className="mt-0.5 space-y-0.5 pl-1">
                  {group.items.map((item) => {
                    const active = pathname === item.href || pathname.startsWith(item.href + "/");
                    return (
                      <Link
                        key={item.href}
                        href={item.href}
                        className={`block rounded-[4px] border-l-[3px] px-2.5 py-1.5 text-[13px] transition-colors duration-150 ${
                          active
                            ? "border-[#2E7BF6] bg-[#26374F] font-medium text-white"
                            : "border-transparent text-[#C4CDD9] hover:bg-[#26374F] hover:text-white"
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
          <div className="px-3 py-2 text-xs text-[#7E8CA0]">No matches</div>
        )}
      </nav>

      <div className="border-t border-white/10 px-3 py-3">
        <div className="mb-2 truncate px-1 text-xs text-[#7E8CA0]">
          {username ? <>Signed in as <span className="text-[#C4CDD9]">{username}</span></> : " "}
        </div>
        <button
          onClick={logout}
          className="flex w-full items-center gap-2.5 rounded-[4px] px-3 py-1.5 text-[13px] text-[#C4CDD9] transition-colors duration-150 hover:bg-[#26374F] hover:text-white"
        >
          <LogOut size={14} strokeWidth={2} />
          Log out
        </button>
      </div>
    </aside>
  );
}
