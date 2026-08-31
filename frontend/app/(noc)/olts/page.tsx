"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "../../../lib/api";

type OLT = { id: string; name: string; address: string; vendor: string; model?: string; serial?: string; enabled: boolean };
type RuntimeState = { oltId: string; running: boolean; lastPollAt?: string; lastError?: string; pollCount: number };
type CreateResponse = { olt: OLT; polling: boolean; warning?: string };

const input = "mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 outline-none focus:border-cyan-500";

export default function OLTsPage() {
  const [olts, setOlts] = useState<OLT[]>([]);
  const [runtime, setRuntime] = useState<RuntimeState[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  async function loadOLTs() {
    setLoading(true);
    try {
      const [items, runtimeResult] = await Promise.all([
        apiFetch<OLT[]>("/olts"),
        apiFetch<{ olts: RuntimeState[] }>("/olt/runtime"),
      ]);
      setOlts(items);
      setRuntime(runtimeResult.olts || []);
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Unable to load OLT inventory.");
    } finally { setLoading(false); }
  }
  useEffect(() => { loadOLTs(); const timer = setInterval(loadOLTs, 15000); return () => clearInterval(timer); }, []);

  async function saveOLT(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setMessage("");
    try {
      const data = new FormData(event.currentTarget);
      const body = {
        name: data.get("name"), address: data.get("address"), vendor: data.get("vendor"),
        model: data.get("model"), serial: data.get("serial"), snmpVersion: data.get("snmpVersion"),
        snmpCommunity: data.get("snmpCommunity"), pollIntervalSeconds: Number(data.get("pollIntervalSeconds") || 60),
      };
      const result = await apiFetch<CreateResponse>("/olts", { method: "POST", body: JSON.stringify(body) });
      setMessage(result.warning ? `OLT saved. ${result.warning}` : "OLT saved and polling started.");
      event.currentTarget.reset(); await loadOLTs();
    } catch (err) { setMessage(err instanceof ApiError ? err.message : "Failed to save OLT."); }
    finally { setSaving(false); }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-widest text-cyan-400">FIBER OPERATIONS / MONITORING</div>
        <h1 className="mt-2 text-3xl font-bold">OLT Inventory</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-400">Register OLTs, configure SNMP credentials, start the live poller, and open an OLT to inspect PONs, ONUs, optical health and alerts.</p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-slate-700 bg-slate-900 px-4 py-3 text-sm text-cyan-300">{message}</div>}
      <section className="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4"><Stat label="Configured" value={olts.length}/><Stat label="Pollers running" value={runtime.filter(x => x.running).length}/><Stat label="Pollers stopped" value={runtime.filter(x => !x.running).length}/><Stat label="Enabled" value={olts.filter(x => x.enabled).length}/></section>
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="rounded-xl border border-slate-800 bg-slate-900 p-5 lg:col-span-2">
          <div className="flex items-center justify-between"><div><h2 className="font-semibold">Configured OLTs</h2><p className="mt-1 text-xs text-slate-500">Runtime status is read directly from the backend polling manager.</p></div><button onClick={loadOLTs} className="rounded border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Refresh</button></div>
          <div className="mt-4 overflow-x-auto"><table className="w-full min-w-[900px] text-left text-sm"><thead className="border-b border-slate-800 text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-3 py-3">Name</th><th>Address</th><th>Vendor</th><th>Model</th><th>SNMP</th><th>Poller</th><th>Action</th></tr></thead><tbody>
          {loading?<tr><td colSpan={7} className="px-3 py-8 text-center text-slate-500">Loading…</td></tr>:olts.length?olts.map(o=>{const r=runtime.find(x=>x.oltId===o.id);return <tr key={o.id} className="border-b border-slate-800/70 hover:bg-slate-950/50"><td className="px-3 py-3 font-medium"><Link href={`/olts/${o.id}`} className="text-cyan-400 hover:underline">{o.name}</Link></td><td className="text-slate-300">{o.address}</td><td className="uppercase text-xs">{o.vendor}</td><td className="text-slate-400">{o.model||"—"}</td><td><span className="rounded-full border border-cyan-900 bg-cyan-950/40 px-2 py-1 text-xs text-cyan-300">SNMP</span></td><td><span className={`rounded-full border px-2 py-1 text-xs ${r?.running?'border-emerald-900 bg-emerald-950 text-emerald-300':'border-amber-900 bg-amber-950 text-amber-300'}`}>{r?.running?'Running':'Stopped'}</span>{r?.lastPollAt&&<div className="mt-1 text-[11px] text-slate-500">{new Date(r.lastPollAt).toLocaleTimeString()} · {r.pollCount} polls</div>}{r?.lastError&&<div className="mt-1 max-w-xs truncate text-[11px] text-red-300" title={r.lastError}>{r.lastError}</div>}</td><td><Link href={`/olts/${o.id}`} className="rounded-lg border border-slate-700 px-3 py-2 text-xs text-cyan-300 hover:bg-slate-800">Open telemetry</Link></td></tr>}) : <tr><td colSpan={7} className="px-3 py-8 text-center text-slate-500">No OLTs configured yet.</td></tr>}
          </tbody></table></div>
        </section>
        <section className="rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Add OLT</h2><p className="mt-1 text-xs text-slate-500">New OLTs are saved and the backend attempts to start polling immediately.</p><form onSubmit={saveOLT} className="mt-4 space-y-4"><label className="block text-sm text-slate-300">Name<input required name="name" placeholder="OLT-Central-01" className={input}/></label><label className="block text-sm text-slate-300">IP address<input required name="address" placeholder="10.0.1.1" className={input}/></label><label className="block text-sm text-slate-300">Vendor<select name="vendor" className={input}><option value="zte">ZTE</option><option value="huawei">Huawei</option><option value="fiberhome">FiberHome</option><option value="other">Other (manual polling only)</option></select></label><label className="block text-sm text-slate-300">Model<input name="model" placeholder="C320" className={input}/></label><label className="block text-sm text-slate-300">Serial<input name="serial" className={input}/></label><label className="block text-sm text-slate-300">SNMP version<select name="snmpVersion" className={input}><option value="2c">SNMP v2c</option><option value="3">SNMP v3</option></select></label><label className="block text-sm text-slate-300">SNMP community<input name="snmpCommunity" type="password" placeholder="public" className={input}/></label><label className="block text-sm text-slate-300">Poll interval (seconds)<input name="pollIntervalSeconds" type="number" defaultValue="60" min={30} className={input}/></label><button disabled={saving} className="w-full rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-50">{saving?"Saving…":"Add OLT & Start Polling"}</button></form></section>
      </div>
    </main>
  );
}
function Stat({label,value}:{label:string;value:number}){return <div className="rounded-xl border border-slate-800 bg-slate-900 p-5"><div className="text-xs uppercase tracking-wide text-slate-500">{label}</div><div className="mt-2 text-2xl font-bold">{value}</div></div>}
