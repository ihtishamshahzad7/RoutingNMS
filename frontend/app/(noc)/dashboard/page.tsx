"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiFetch, ApiError } from "../../../lib/api";
import BackendSync from "./backend-sync";
import { StatCard } from "../../../components/ui/card";
import { StatusPill } from "../../../components/ui/status-pill";
import { StatusDot } from "../../../components/ui/status-dot";
import { Button } from "../../../components/ui/primitives";

type RuntimeState={oltId:string;running:boolean;startedAt?:string;lastPollAt?:string;lastError?:string;pollCount:number};
type Alert={id:number;oltId:string;ponId:string;onuId:string;code:string;severity:string;message:string;status:string;lastSeen:string};
type DeviceHealth={id:string;name:string;address:string;deviceType:string;vendor?:string;method:"SNMP"|"TCP";reachable:boolean;latencyMs:number;checkedAt:string;error?:string};
const ORG="tenant-1";
function formatTime(v?:string){return v?new Date(v).toLocaleString():"\u2014"}

export default function DashboardPage(){
 const router=useRouter();const [states,setStates]=useState<RuntimeState[]>([]);const [alerts,setAlerts]=useState<Alert[]>([]);const [health,setHealth]=useState<DeviceHealth[]>([]);const [healthLoading,setHealthLoading]=useState(true);
 const loadHealth=async()=>{setHealthLoading(true);try{setHealth(await apiFetch<DeviceHealth[]>(`/devices/health?organizationId=${ORG}`))}catch(e){if(e instanceof ApiError&&e.status===401)router.replace("/")}finally{setHealthLoading(false)}};
 useEffect(()=>{let active=true;const load=async()=>{try{const data=await apiFetch<{olts:RuntimeState[]}>("/olt/runtime");if(active)setStates(Array.isArray(data.olts)?data.olts:[])}catch(e){if(active&&e instanceof ApiError&&e.status===401)router.replace("/")}};load();loadHealth();const t=window.setInterval(()=>{load();loadHealth()},15000);return()=>{active=false;window.clearInterval(t)}},[router]);
 useEffect(()=>{let active=true;const load=async()=>{const results=await Promise.all(states.filter(s=>s.oltId).map(async s=>{try{return await apiFetch<Alert[]>(`/olts/${encodeURIComponent(s.oltId)}/alerts?limit=10`)}catch{return[]}}));if(active)setAlerts(results.flat().filter((a:Alert)=>a.status==="open").slice(0,20))};if(states.length)load();else setAlerts([]);return()=>{active=false}},[states]);
 const running=states.filter(s=>s.running).length;const critical=alerts.filter(a=>a.severity.toLowerCase()==="critical").length;const healthy=health.filter(d=>d.reachable).length;const totalPolls=useMemo(()=>states.reduce((n,s)=>n+s.pollCount,0),[states]);

 return (
  <main className="mx-auto max-w-7xl px-6 py-6">
   {/* Page header */}
   <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
    <div>
     <div className="label text-[#8B949E]">Live NOC</div>
     <h1 className="mt-1 text-[22px] font-bold tracking-[-0.5px] text-[#E6EDF3]">Network Overview</h1>
     <p className="mt-1 text-xs text-[#8B949E]">One operational view for device reachability, SNMP monitoring, OLT runtime and incidents.</p>
    </div>
    <Button variant="secondary" onClick={loadHealth} disabled={healthLoading}>
     {healthLoading ? "Checking devices…" : "Check devices now"}
    </Button>
   </div>

   {/* Stat cards */}
   <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
    <StatCard label="Device Health" value={health.length ? `${healthy}/${health.length}` : "—"} sub="Reachable devices" accent={healthy === health.length && health.length ? "text-[#3FB950]" : "text-[#D29922]"} />
    <StatCard label="OLT Pollers" value={running} sub={`${totalPolls} polls completed`} accent="text-[#58A6FF]" />
    <StatCard label="Critical Alerts" value={critical} sub={critical ? "Immediate attention required" : "No active critical alerts"} accent={critical ? "text-[#F78166]" : "text-[#3FB950]"} />
    <StatCard label="Open Alerts" value={alerts.length} sub={alerts.length ? "Active incidents" : "No active incidents"} accent={alerts.length ? "text-[#D29922]" : "text-[#3FB950]"} />
   </div>

   {/* Device health table */}
   <section className="mt-5 rounded-[8px] border border-[#21262D] bg-[#161B22]">
    <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
     <div>
      <h2 className="text-base font-semibold text-[#E6EDF3]">Device health</h2>
      <p className="mt-0.5 text-[10px] text-[#8B949E]">Live backend probes. SNMP-enabled devices are checked through their configured SNMP endpoint; other devices use TCP reachability.</p>
     </div>
     <Link href="/devices" className="text-xs text-[#58A6FF] hover:underline">Manage devices →</Link>
    </div>
    <div className="overflow-x-auto">
     <table className="tbl w-full min-w-[760px] text-left text-xs">
      <thead><tr><th>Device</th><th>Address</th><th>Method</th><th>Status</th><th>Latency</th><th>Checked</th></tr></thead>
      <tbody>
       {health.length ? health.map(d=>(
        <tr key={d.id}>
         <td className="text-[#E6EDF3]"><Link href={`/devices/${d.id}`} className="font-medium text-[#58A6FF] hover:underline">{d.name}</Link><div className="text-[10px] text-[#8B949E]">{d.vendor||d.deviceType}</div></td>
         <td className="mono">{d.address}</td>
         <td><StatusPill status={d.method} label={d.method} /></td>
         <td><StatusPill status={d.reachable ? "up" : "down"} label={d.reachable ? "Reachable" : "Down"} pulse={!d.reachable} />{d.error&&<div className="mt-1 max-w-xs truncate text-[10px] text-[#F78166]" title={d.error}>{d.error}</div>}</td>
         <td className="mono text-[#8B949E]">{d.latencyMs.toFixed(1)} ms</td>
         <td className="text-[#484F58]">{formatTime(d.checkedAt)}</td>
        </tr>
       )) : <tr><td colSpan={6} className="py-10 text-center text-[#8B949E]">{healthLoading ? "Checking registered devices…" : "No registered devices yet."}</td></tr>}
      </tbody>
     </table>
    </div>
   </section>

   {/* OLT Runtime + Active Alerts */}
   <div className="mt-5 grid gap-5 lg:grid-cols-3">
    <section className="rounded-[8px] border border-[#21262D] bg-[#161B22] p-4 lg:col-span-2">
     <div className="flex justify-between">
      <h2 className="text-base font-semibold text-[#E6EDF3]">OLT Runtime</h2>
      <Link href="/olts" className="text-xs text-[#58A6FF] hover:underline">Manage OLTs →</Link>
     </div>
     <div className="mt-3 space-y-2">
      {states.length ? states.map(s=>(
       <div key={s.oltId} className="rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3">
        <div className="flex items-center justify-between">
         <div className="flex items-center gap-2 font-medium text-[#E6EDF3]"><StatusDot status={s.running ? "up" : "down"} pulse={!s.running} />{s.oltId}</div>
         <StatusPill status={s.running ? "up" : "down"} label={s.running ? "Running" : "Stopped"} pulse={!s.running} />
        </div>
        <div className="mt-2 grid gap-2 text-[10px] text-[#8B949E] sm:grid-cols-3">
         <span>Polls: {s.pollCount}</span>
         <span>Last poll: {formatTime(s.lastPollAt)}</span>
         <span className={s.lastError ? "text-[#F78166]" : "text-[#3FB950]"}>{s.lastError ? `Error: ${s.lastError}` : "Healthy"}</span>
        </div>
       </div>
      )) : <div className="flex min-h-32 items-center justify-center rounded-[6px] border border-dashed border-[#21262D] text-xs text-[#8B949E]">No OLT runtime data available. <Link href="/olts" className="ml-1 text-[#58A6FF]">Add an OLT</Link></div>}
     </div>
    </section>

    <section className="rounded-[8px] border border-[#21262D] bg-[#161B22] p-4">
     <h2 className="text-base font-semibold text-[#E6EDF3]">Active Alerts</h2>
     <div className="mt-3 space-y-2">
      {alerts.length ? alerts.map(a=>(
       <div key={a.id} className="rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3">
        <div className="flex items-center justify-between gap-2">
         <StatusPill status={a.severity} label={a.severity} pulse={a.severity.toLowerCase() === "critical"} />
         <span className="mono text-[#484F58]">{a.oltId}</span>
        </div>
        <div className="mt-1 text-xs text-[#E6EDF3]">{a.message}</div>
        <div className="mt-1 text-[10px] text-[#8B949E]">{a.onuId||a.ponId||a.code} · {formatTime(a.lastSeen)}</div>
       </div>
      )) : <div className="rounded-[6px] border border-[#21262D] p-3 text-xs text-[#8B949E]">No active alerts.</div>}
     </div>
    </section>
   </div>

   {/* Infrastructure */}
   <section className="mt-5 rounded-[8px] border border-[#21262D] bg-[#161B22] p-4">
    <h2 className="text-base font-semibold text-[#E6EDF3]">Infrastructure</h2>
    <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
     <Link href="/devices" className="rounded-[6px] bg-[#0D1117] p-3 transition-colors duration-100 hover:bg-[#1C2128]"><div className="text-xs text-[#8B949E]">Routers / Switches</div><div className="mt-1 text-xs text-[#58A6FF]">Manage devices →</div></Link>
     <Link href="/devices" className="rounded-[6px] bg-[#0D1117] p-3 transition-colors duration-100 hover:bg-[#1C2128]"><div className="text-xs text-[#8B949E]">SNMP</div><div className="mt-1 text-xs text-[#58A6FF]">Configure & monitor →</div></Link>
     <Link href="/olts" className="rounded-[6px] bg-[#0D1117] p-3 transition-colors duration-100 hover:bg-[#1C2128]"><div className="text-xs text-[#8B949E]">OLTs</div><div className="mt-1 text-xs text-[#58A6FF]">Configure →</div></Link>
     <Link href="/incidents" className="rounded-[6px] bg-[#0D1117] p-3 transition-colors duration-100 hover:bg-[#1C2128]"><div className="text-xs text-[#8B949E]">Incidents</div><div className="mt-1 text-xs text-[#58A6FF]">View alerts →</div></Link>
    </div>
   </section>

   <BackendSync />
  </main>
 );
}