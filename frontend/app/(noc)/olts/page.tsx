"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "../../../lib/api";

type OLT = { id: string; name: string; address: string; vendor: string; model?: string; serial?: string; enabled: boolean };
type CreateResponse = { olt: OLT; polling: boolean; warning?: string };

const input = "mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 outline-none focus:border-cyan-500";

export default function OLTsPage() {
  const [olts, setOlts] = useState<OLT[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  async function loadOLTs() {
    setLoading(true);
    try { setOlts(await apiFetch<OLT[]>("/olts")); }
    catch (err) { setMessage(err instanceof ApiError ? err.message : "Unable to load OLTs."); }
    finally { setLoading(false); }
  }
  useEffect(() => { loadOLTs(); }, []);

  async function saveOLT(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setMessage("");
    try {
      const data = new FormData(event.currentTarget);
      const body = {
        name: data.get("name"),
        address: data.get("address"),
        vendor: data.get("vendor"),
        model: data.get("model"),
        serial: data.get("serial"),
        snmpVersion: data.get("snmpVersion"),
        snmpCommunity: data.get("snmpCommunity"),
        pollIntervalSeconds: Number(data.get("pollIntervalSeconds") || 60),
      };
      const result = await apiFetch<CreateResponse>("/olts", { method: "POST", body: JSON.stringify(body) });
      setMessage(result.warning ? `OLT saved. ${result.warning}` : "OLT saved and polling started.");
      event.currentTarget.reset();
      await loadOLTs();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save OLT.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-widest text-cyan-400">FIBER OPERATIONS</div>
        <h1 className="mt-2 text-3xl font-bold">OLTs</h1>
        <p className="mt-2 text-sm text-slate-400">Register an OLT to start polling it over SNMP. Only ZTE, Huawei and FiberHome ship a built-in profile today — other vendors save but do not poll automatically.</p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-slate-700 bg-slate-900 px-4 py-3 text-sm text-cyan-300">{message}</div>}
      <div className="grid gap-6 lg:grid-cols-3">
        <section className="rounded-xl border border-slate-800 bg-slate-900 p-5 lg:col-span-2">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold">Configured OLTs</h2>
            <button onClick={loadOLTs} className="rounded border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Refresh</button>
          </div>
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-slate-800 text-xs text-slate-500">
                <tr><th className="px-3 py-3">Name</th><th className="px-3 py-3">Address</th><th className="px-3 py-3">Vendor</th><th className="px-3 py-3">Model</th><th className="px-3 py-3">Status</th></tr>
              </thead>
              <tbody>
                {loading ? (
                  <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">Loading…</td></tr>
                ) : olts.length ? olts.map(o => (
                  <tr key={o.id} className="border-b border-slate-800/70">
                    <td className="px-3 py-3 font-medium"><Link href={`/olts/${o.id}`} className="text-cyan-400 hover:underline">{o.name}</Link></td>
                    <td className="px-3 py-3 text-slate-300">{o.address}</td>
                    <td className="px-3 py-3 uppercase text-xs">{o.vendor}</td>
                    <td className="px-3 py-3 text-slate-400">{o.model || "—"}</td>
                    <td className="px-3 py-3"><span className="rounded-full bg-emerald-950 px-2 py-1 text-xs text-emerald-300">{o.enabled ? "Enabled" : "Disabled"}</span></td>
                  </tr>
                )) : (
                  <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">No OLTs configured yet.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
        <section className="rounded-xl border border-slate-800 bg-slate-900 p-5">
          <h2 className="font-semibold">Add OLT</h2>
          <form onSubmit={saveOLT} className="mt-4 space-y-4">
            <label className="block text-sm text-slate-300">Name<input required name="name" placeholder="OLT-Central-01" className={input} /></label>
            <label className="block text-sm text-slate-300">IP address<input required name="address" placeholder="10.0.1.1" className={input} /></label>
            <label className="block text-sm text-slate-300">Vendor
              <select name="vendor" className={input}>
                <option value="zte">ZTE</option>
                <option value="huawei">Huawei</option>
                <option value="fiberhome">FiberHome</option>
                <option value="other">Other (manual polling only)</option>
              </select>
            </label>
            <label className="block text-sm text-slate-300">Model<input name="model" placeholder="C320" className={input} /></label>
            <label className="block text-sm text-slate-300">Serial<input name="serial" className={input} /></label>
            <label className="block text-sm text-slate-300">SNMP version
              <select name="snmpVersion" className={input}>
                <option value="2c">SNMP v2c</option>
                <option value="3">SNMP v3</option>
              </select>
            </label>
            <label className="block text-sm text-slate-300">SNMP community<input name="snmpCommunity" type="password" placeholder="public" className={input} /></label>
            <label className="block text-sm text-slate-300">Poll interval (seconds)<input name="pollIntervalSeconds" type="number" defaultValue="60" min={30} className={input} /></label>
            <button disabled={saving} className="w-full rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-50">{saving ? "Saving…" : "Add OLT"}</button>
          </form>
        </section>
      </div>
    </main>
  );
}
