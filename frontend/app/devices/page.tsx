"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "../../lib/api";

type Device = { id: string; organizationId: string; name: string; address: string; deviceType: string; vendor?: string; model?: string; serialNumber?: string; enabled: boolean; monitoringIntervalSeconds: number };
type TestResult = { reachable: boolean; systemName?: string; sysDescr?: string; interfaceCount?: number; error?: string };

const ORG = "tenant-1";
const input = "mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 outline-none focus:border-cyan-500";

export default function DevicesPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [message, setMessage] = useState("");
  const [result, setResult] = useState<TestResult | null>(null);

  async function loadDevices() {
    setLoading(true); setMessage("");
    try { setDevices(await apiFetch<Device[]>(`/devices?organizationId=${ORG}`)); }
    catch (err) { setMessage(err instanceof ApiError ? err.message : "Unable to load devices."); }
    finally { setLoading(false); }
  }
  useEffect(() => { loadDevices(); }, []);

  function payload(data: FormData) {
    return { organizationId: ORG, name: data.get("name"), address: data.get("address"), deviceType: data.get("deviceType"), vendor: data.get("vendor"), snmpPort: Number(data.get("snmpPort") || 161), timeoutMs: Number(data.get("timeoutMs") || 3000), snmp: { version: data.get("version"), community: data.get("community") } };
  }

  async function testConnection(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setTesting(true); setResult(null); setMessage("");
    try { setResult(await apiFetch<TestResult>("/devices/test", { method: "POST", body: JSON.stringify(payload(new FormData(event.currentTarget))) })); }
    catch (err) { setResult({ reachable: false, error: err instanceof ApiError ? err.message : "Unable to reach the backend." }); }
    finally { setTesting(false); }
  }

  async function saveDevice(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setMessage("");
    try {
      const data = new FormData(event.currentTarget);
      const body = { organizationId: ORG, name: data.get("name"), address: data.get("address"), deviceType: data.get("deviceType"), vendor: data.get("vendor") };
      await apiFetch<Device>("/devices", { method: "POST", body: JSON.stringify(body) });
      setMessage("Device registered successfully."); event.currentTarget.reset(); await loadDevices();
    } catch (err) { setMessage(err instanceof ApiError ? err.message : "Failed to register device."); }
    finally { setSaving(false); }
  }

  return (
    <main className="min-h-screen bg-slate-950 p-6 text-slate-100">
      <div className="mx-auto max-w-7xl">
        <nav className="mb-8 flex flex-wrap items-center gap-3 border-b border-slate-800 pb-4 text-sm">
          <Link href="/dashboard" className="font-bold text-cyan-400">RoutingNMS</Link><Link href="/dashboard" className="text-slate-400 hover:text-white">Dashboard</Link><Link href="/devices" className="rounded bg-slate-800 px-3 py-1 text-white">Devices</Link><Link href="/olts" className="text-slate-400 hover:text-white">OLTs</Link><Link href="/incidents" className="text-slate-400 hover:text-white">Incidents</Link><Link href="/topology" className="text-slate-400 hover:text-white">Topology</Link>
        </nav>
        <div className="mb-8"><div className="text-xs font-semibold tracking-widest text-cyan-400">INVENTORY</div><h1 className="mt-2 text-3xl font-bold">Network Devices</h1><p className="mt-2 text-sm text-slate-400">Register routers, switches, OLTs and SNMP-capable infrastructure, then test SNMP before monitoring.</p></div>
        {message && <div className="mb-5 rounded-lg border border-slate-700 bg-slate-900 px-4 py-3 text-sm text-cyan-300">{message}</div>}
        <div className="grid gap-6 lg:grid-cols-3">
          <section className="rounded-xl border border-slate-800 bg-slate-900 p-5 lg:col-span-2">
            <div className="flex items-center justify-between"><h2 className="font-semibold">Registered devices</h2><button onClick={loadDevices} className="rounded border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Refresh</button></div>
            <div className="mt-4 overflow-x-auto"><table className="w-full text-left text-sm"><thead className="border-b border-slate-800 text-xs text-slate-500"><tr><th className="px-3 py-3">Name</th><th className="px-3 py-3">Address</th><th className="px-3 py-3">Type</th><th className="px-3 py-3">Vendor</th><th className="px-3 py-3">Status</th></tr></thead><tbody>{loading ? <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">Loading…</td></tr> : devices.length ? devices.map(d => <tr key={d.id} className="border-b border-slate-800/70"><td className="px-3 py-3 font-medium">{d.name}</td><td className="px-3 py-3 text-slate-300">{d.address}</td><td className="px-3 py-3 uppercase text-xs">{d.deviceType}</td><td className="px-3 py-3 text-slate-400">{d.vendor || "—"}</td><td className="px-3 py-3"><span className="rounded-full bg-emerald-950 px-2 py-1 text-xs text-emerald-300">{d.enabled ? "Enabled" : "Disabled"}</span></td></tr>) : <tr><td colSpan={5} className="px-3 py-8 text-center text-slate-500">No devices registered yet.</td></tr>}</tbody></table></div>
          </section>
          <section className="rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Register device</h2><form onSubmit={saveDevice} className="mt-4 space-y-4"><label className="block text-sm text-slate-300">Device name<input required name="name" placeholder="Core-Switch-01" className={input}/></label><label className="block text-sm text-slate-300">IP address<input required name="address" placeholder="10.0.0.1" className={input}/></label><label className="block text-sm text-slate-300">Type<select name="deviceType" className={input}><option>router</option><option>switch</option><option>olt</option><option>server</option><option>other</option></select></label><label className="block text-sm text-slate-300">Vendor<input name="vendor" placeholder="MikroTik / Huawei / ZTE" className={input}/></label><button disabled={saving} className="w-full rounded-lg bg-cyan-600 px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-50">{saving ? "Saving…" : "Add device"}</button></form></section>
        </div>
        <section className="mt-6 rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">SNMP connectivity test</h2><p className="mt-1 text-sm text-slate-500">Use this before registration when you need to verify credentials and interface discovery.</p><form onSubmit={testConnection} className="mt-5 grid gap-4 md:grid-cols-2 lg:grid-cols-4"><label className="text-sm text-slate-300">Name<input required name="name" placeholder="Core-Router-01" className={input}/></label><label className="text-sm text-slate-300">IP address<input required name="address" placeholder="10.0.0.1" className={input}/></label><label className="text-sm text-slate-300">Type<select name="deviceType" className={input}><option>router</option><option>switch</option><option>olt</option></select></label><label className="text-sm text-slate-300">Vendor<input name="vendor" placeholder="MikroTik" className={input}/></label><label className="text-sm text-slate-300">SNMP version<select name="version" className={input}><option value="2c">SNMP v2c</option><option value="3">SNMP v3</option></select></label><label className="text-sm text-slate-300">Community<input name="community" type="password" placeholder="public" className={input}/></label><label className="text-sm text-slate-300">SNMP port<input name="snmpPort" type="number" defaultValue="161" className={input}/></label><label className="text-sm text-slate-300">Timeout<input name="timeoutMs" type="number" defaultValue="3000" className={input}/></label><div className="md:col-span-2 lg:col-span-4"><button disabled={testing} className="rounded-lg border border-cyan-700 bg-cyan-950 px-5 py-2.5 text-sm font-semibold text-cyan-300 disabled:opacity-50">{testing ? "Testing SNMP…" : "Test SNMP connection"}</button></div></form>{result && <div className="mt-5 rounded-lg border border-slate-800 bg-slate-950 p-4 text-sm"><strong className={result.reachable ? "text-emerald-400" : "text-red-400"}>{result.reachable ? "Reachable" : "Failed"}</strong>{result.systemName && <span className="ml-4 text-slate-300">System: {result.systemName}</span>}{result.interfaceCount !== undefined && <span className="ml-4 text-slate-300">Interfaces: {result.interfaceCount}</span>}{result.error && <pre className="mt-2 whitespace-pre-wrap text-xs text-red-300">{result.error}</pre>}</div>}</section>
      </div>
    </main>
  );
}
