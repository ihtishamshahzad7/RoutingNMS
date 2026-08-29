"use client";

import { FormEvent, useState } from "react";

type TestResult = { reachable: boolean; systemName?: string; sysDescr?: string; interfaceCount?: number; error?: string };

export default function DevicesPage() {
  const [result, setResult] = useState<TestResult | null>(null);
  const [testing, setTesting] = useState(false);

  async function testConnection(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setTesting(true);
    setResult(null);
    const data = new FormData(event.currentTarget);
    const payload = {
      organizationId: data.get("organizationId"), name: data.get("name"), address: data.get("address"),
      deviceType: data.get("deviceType"), vendor: data.get("vendor"), snmpPort: Number(data.get("snmpPort") || 161),
      timeoutMs: Number(data.get("timeoutMs") || 3000), snmp: { version: data.get("version"), community: data.get("community") },
    };
    try {
      const response = await fetch("/api/devices/test", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
      const body = await response.json();
      setResult(body);
    } catch { setResult({ reachable: false, error: "Unable to reach the backend." }); }
    finally { setTesting(false); }
  }

  return (
    <main className="min-h-screen bg-slate-950 p-6 text-slate-100">
      <div className="mx-auto max-w-4xl">
        <div className="mb-8"><div className="text-xs font-semibold tracking-widest text-cyan-400">INVENTORY</div><h1 className="mt-2 text-3xl font-bold">Add network device</h1><p className="mt-2 text-sm text-slate-400">Test SNMP before registering a router, switch or OLT.</p></div>
        <form onSubmit={testConnection} className="grid gap-5 rounded-xl border border-slate-800 bg-slate-900 p-6 md:grid-cols-2">
          {[['organizationId','Organization ID','tenant-1'],['name','Device name','Core-Router-01'],['address','IP address','10.0.0.1'],['vendor','Vendor','MikroTik']].map(([name,label,placeholder]) => <label key={name} className="block"><span className="text-sm text-slate-300">{label}</span><input required name={name} placeholder={placeholder} className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 outline-none focus:border-cyan-500" /></label>)}
          <label><span className="text-sm text-slate-300">Device type</span><select name="deviceType" className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2"><option>router</option><option>switch</option><option>olt</option><option>server</option></select></label>
          <label><span className="text-sm text-slate-300">SNMP version</span><select name="version" className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2"><option value="2c">SNMP v2c</option><option value="3">SNMP v3</option></select></label>
          <label><span className="text-sm text-slate-300">Community</span><input name="community" type="password" placeholder="SNMP community" className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" /></label>
          <label><span className="text-sm text-slate-300">SNMP port</span><input name="snmpPort" type="number" defaultValue="161" className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" /></label>
          <label><span className="text-sm text-slate-300">Timeout (ms)</span><input name="timeoutMs" type="number" defaultValue="3000" className="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" /></label>
          <div className="md:col-span-2"><button disabled={testing} className="rounded-lg bg-cyan-600 px-5 py-2.5 text-sm font-semibold disabled:opacity-50">{testing ? "Testing SNMP…" : "Test SNMP connection"}</button></div>
        </form>
        {result && <section className="mt-5 rounded-xl border border-slate-800 bg-slate-900 p-5"><h2 className="font-semibold">Test result</h2><div className="mt-3 text-sm">Status: <strong className={result.reachable ? "text-emerald-400" : "text-red-400"}>{result.reachable ? "Reachable" : "Failed"}</strong></div>{result.systemName && <div className="mt-2 text-sm text-slate-300">System: {result.systemName}</div>}{result.interfaceCount !== undefined && <div className="mt-2 text-sm text-slate-300">Interfaces discovered: {result.interfaceCount}</div>}{result.error && <pre className="mt-3 whitespace-pre-wrap text-xs text-red-300">{result.error}</pre>}</section>}
      </div>
    </main>
  );
}
