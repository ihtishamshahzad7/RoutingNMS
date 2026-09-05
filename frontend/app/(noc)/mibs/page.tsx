"use client";

import { useEffect, useState, FormEvent, ChangeEvent } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

const ORG = "tenant-1";

type MIB = {
  id: number;
  filename: string;
  moduleName?: string;
  objectCount: number;
  skippedCount: number;
  uploadedAt: string;
};

type SearchResult = { name: string; oid: string; mibId: number; filename: string };
type Device = { id: string; name: string; address: string; snmpEnabled: boolean };
type TestResult = { oid: string; resolvedName?: string; value: unknown };

export default function MIBsPage() {
  const [mibs, setMibs] = useState<MIB[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  const [testDevice, setTestDevice] = useState("");
  const [testOID, setTestOID] = useState("");
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [testing, setTesting] = useState(false);

  async function load() {
    setLoading(true);
    setError("");
    try {
      const [mibsRes, devicesRes] = await Promise.all([
        apiFetch<MIB[]>("/mibs"),
        apiFetch<Device[]>(`/devices?organizationId=${ORG}`),
      ]);
      setMibs(mibsRes);
      setDevices(devicesRes.filter(d => d.snmpEnabled));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Unable to load MIBs.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function onUpload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setMessage("");
    setError("");
    try {
      const form = new FormData();
      form.append("file", file);
      // Deliberately bypasses apiFetch here: it always sends
      // Content-Type: application/json, which would strip the multipart
      // boundary the browser needs to set itself for a FormData body.
      const res = await fetch(`/api/v1/mibs?filename=${encodeURIComponent(file.name)}`, {
        method: "POST",
        credentials: "include",
        body: form,
      });
      if (!res.ok) {
        const body = await res.json().catch(() => null);
        throw new ApiError(res.status, body?.error || `Upload failed with status ${res.status}`);
      }
      setMessage(`Uploaded ${file.name}.`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to upload MIB.");
    } finally {
      setUploading(false);
      event.target.value = "";
    }
  }

  async function deleteMib(id: number) {
    try {
      await apiFetch(`/mibs/${id}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete MIB.");
    }
  }

  async function search(event: FormEvent) {
    event.preventDefault();
    setSearching(true);
    setError("");
    try {
      setResults(await apiFetch<SearchResult[]>(`/mibs/search?q=${encodeURIComponent(query)}`));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Search failed.");
    } finally {
      setSearching(false);
    }
  }

  async function runTest(event: FormEvent) {
    event.preventDefault();
    setTesting(true);
    setTestResult(null);
    setError("");
    try {
      setTestResult(await apiFetch<TestResult>("/mibs/test", {
        method: "POST",
        body: JSON.stringify({ deviceId: testDevice, oid: testOID }),
      }));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "OID test failed.");
    } finally {
      setTesting(false);
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-widest text-[#58A6FF]">LIVE NOC</div>
        <h1 className="mt-2 text-3xl font-bold">MIB Manager</h1>
        <p className="mt-2 text-sm text-[#8B949E]">
          Upload vendor .mib/.my files to resolve raw OIDs into readable names across traps, alerts and this tester.
        </p>
      </div>

      {error && <div className="mb-5 rounded-[6px] border border-[#672525] bg-[#2D1212] px-4 py-3 text-sm text-[#F78166]">{error}</div>}
      {message && <div className="mb-5 rounded-[6px] border border-[#2EA043] bg-[#12261E]/40 px-4 py-3 text-sm text-[#3FB950]">{message}</div>}

      <section className="mb-8 rounded-[8px] border border-[#21262D] bg-[#161B22]">
        <div className="flex items-center justify-between border-b border-[#21262D] px-5 py-4">
          <h2 className="text-sm font-semibold text-[#E6EDF3]">Loaded MIBs</h2>
          <label className="cursor-pointer rounded border border-[#30363D] px-3 py-1.5 text-xs hover:bg-[#21262D]">
            {uploading ? "Uploading…" : "Upload MIB file"}
            <input type="file" accept=".mib,.my,.txt" className="hidden" onChange={onUpload} disabled={uploading} />
          </label>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-[#21262D] text-xs text-[#8B949E]">
              <tr>
                <th className="px-5 py-3">File</th>
                <th className="px-5 py-3">Module</th>
                <th className="px-5 py-3">Objects resolved</th>
                <th className="px-5 py-3">Uploaded</th>
                <th className="px-5 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr><td colSpan={5} className="px-5 py-6 text-center text-[#8B949E]">Loading…</td></tr>
              ) : mibs.length ? mibs.map(m => (
                <tr key={m.id} className="border-b border-[#21262D]">
                  <td className="px-5 py-3 font-mono text-xs">{m.filename}</td>
                  <td className="px-5 py-3 text-xs text-[#8B949E]">{m.moduleName || "—"}</td>
                  <td className="px-5 py-3 text-xs text-[#8B949E]">
                    {m.objectCount}{m.skippedCount > 0 && <span className="text-[#D29922]"> ({m.skippedCount} skipped — dependency MIB missing?)</span>}
                  </td>
                  <td className="px-5 py-3 text-xs text-[#8B949E]">{new Date(m.uploadedAt).toLocaleString()}</td>
                  <td className="px-5 py-3 text-right">
                    <button onClick={() => deleteMib(m.id)} className="text-xs text-[#F78166] hover:text-[#F78166]">Delete</button>
                  </td>
                </tr>
              )) : (
                <tr><td colSpan={5} className="px-5 py-6 text-center text-[#8B949E]">No MIBs uploaded yet.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-2">
        <section className="rounded-[8px] border border-[#21262D] bg-[#161B22] p-5">
          <h2 className="mb-4 text-sm font-semibold text-[#E6EDF3]">OID search</h2>
          <form onSubmit={search} className="mb-4 flex gap-2">
            <input
              value={query}
              onChange={e => setQuery(e.target.value)}
              placeholder="Search by name or OID (e.g. sysDescr or 1.3.6.1.2.1.1.1)"
              className="flex-1 rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm"
            />
            <button disabled={searching} className="rounded-[6px] bg-[#238636] px-4 py-2 text-sm font-medium text-white hover:bg-[#238636] disabled:opacity-50">
              {searching ? "…" : "Search"}
            </button>
          </form>
          <div className="space-y-2">
            {results.map((r, i) => (
              <div key={i} className="rounded-[6px] border border-[#21262D] px-3 py-2 text-xs">
                <div className="font-medium text-[#E6EDF3]">{r.name}</div>
                <div className="font-mono text-[#8B949E]">{r.oid}</div>
                <div className="text-[#8B949E]">from {r.filename}</div>
              </div>
            ))}
            {!results.length && <div className="text-xs text-[#8B949E]">No results yet — search above.</div>}
          </div>
        </section>

        <section className="rounded-[8px] border border-[#21262D] bg-[#161B22] p-5">
          <h2 className="mb-4 text-sm font-semibold text-[#E6EDF3]">Live OID tester</h2>
          <form onSubmit={runTest} className="space-y-3">
            <select value={testDevice} onChange={e => setTestDevice(e.target.value)} required className="w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm">
              <option value="">Select a device…</option>
              {devices.map(d => <option key={d.id} value={d.id}>{d.name} ({d.address})</option>)}
            </select>
            <input
              value={testOID}
              onChange={e => setTestOID(e.target.value)}
              required
              placeholder="OID to fetch, e.g. 1.3.6.1.2.1.1.1.0"
              className="w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm font-mono"
            />
            <button disabled={testing} className="w-full rounded-[6px] bg-[#238636] px-4 py-2 text-sm font-medium text-white hover:bg-[#238636] disabled:opacity-50">
              {testing ? "Fetching…" : "Fetch value"}
            </button>
          </form>
          {testResult && (
            <div className="mt-4 rounded-[6px] border border-[#21262D] bg-[#0D1117]/60 px-3 py-3 text-xs">
              <div className="text-[#8B949E]">OID</div>
              <div className="mb-2 font-mono text-[#E6EDF3]">{testResult.oid}</div>
              {testResult.resolvedName && <>
                <div className="text-[#8B949E]">Resolved name</div>
                <div className="mb-2 text-[#E6EDF3]">{testResult.resolvedName}</div>
              </>}
              <div className="text-[#8B949E]">Value</div>
              <div className="font-mono text-[#E6EDF3]">{String(testResult.value)}</div>
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
