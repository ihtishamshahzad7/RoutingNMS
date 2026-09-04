"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Window = {
  id: number;
  title: string;
  description: string;
  strategy: "single" | "recurring";
  startsAt?: string | null;
  endsAt?: string | null;
  daysOfWeek?: number[];
  startTimeOfDay?: string | null;
  durationMinutes: number;
  timezone: string;
  active: boolean;
};
type Item = { id?: number; maintenanceWindowId?: number; subjectType: "device" | "olt"; subjectId: string };
type Device = { id: string; name: string };
type Olt = { id: string; name: string };

const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const input = "mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none transition focus:border-cyan-500";
const card = "rounded-2xl border border-slate-800 bg-slate-900 p-5";
const ORG = "tenant-1";

function emptyWindow(): Window {
  return { id: 0, title: "", description: "", strategy: "single", durationMinutes: 60, timezone: "UTC", active: true, daysOfWeek: [] };
}

export default function MaintenanceWindowsAdmin() {
  const [windows, setWindows] = useState<Window[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Window | null>(null);
  const [items, setItems] = useState<Item[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [olts, setOlts] = useState<Olt[]>([]);
  const [saving, setSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      setWindows(await apiFetch<Window[]>("/maintenance-windows"));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load maintenance windows.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);
  useEffect(() => {
    apiFetch<Device[]>(`/devices?organizationId=${ORG}`).then(setDevices).catch(() => {});
    apiFetch<{ id: string; name: string }[]>("/olts").then(setOlts).catch(() => {});
  }, []);

  async function openEditor(w: Window | null) {
    setEditing(w ?? emptyWindow());
    if (w) {
      try {
        const full = await apiFetch<Window & { items: Item[] }>(`/maintenance-windows/${w.id}`);
        setItems(full.items ?? []);
      } catch { setItems([]); }
    } else {
      setItems([]);
    }
  }

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    setSaving(true);
    setMessage("");
    const data = new FormData(e.currentTarget);
    const strategy = String(data.get("strategy")) as "single" | "recurring";
    const daysOfWeek = DOW.map((_, i) => i).filter(i => data.get(`dow-${i}`) === "on");
    const payload: any = {
      title: String(data.get("title") || "").trim(),
      description: String(data.get("description") || ""),
      strategy,
      durationMinutes: Number(data.get("durationMinutes") || 60),
      timezone: String(data.get("timezone") || "UTC").trim() || "UTC",
      active: data.get("active") === "on",
    };
    if (strategy === "single") {
      const startsAt = String(data.get("startsAt") || "");
      const endsAt = String(data.get("endsAt") || "");
      payload.startsAt = startsAt ? new Date(startsAt).toISOString() : null;
      payload.endsAt = endsAt ? new Date(endsAt).toISOString() : null;
    } else {
      payload.daysOfWeek = daysOfWeek;
      payload.startTimeOfDay = String(data.get("startTimeOfDay") || "00:00");
    }
    try {
      let id = editing.id;
      if (id) {
        await apiFetch(`/maintenance-windows/${id}`, { method: "PUT", body: JSON.stringify(payload) });
      } else {
        const created = await apiFetch<Window>("/maintenance-windows", { method: "POST", body: JSON.stringify(payload) });
        id = created.id;
      }
      await apiFetch(`/maintenance-windows/${id}/items`, { method: "PUT", body: JSON.stringify({ items }) });
      setMessage(`Maintenance window "${payload.title}" saved.`);
      setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save maintenance window.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(w: Window) {
    try {
      await apiFetch(`/maintenance-windows/${w.id}`, { method: "DELETE" });
      setMessage(`Maintenance window "${w.title}" deleted.`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete maintenance window.");
    }
  }

  function addItem(type: "device" | "olt", subjectId: string) {
    if (!subjectId) return;
    if (items.some(i => i.subjectType === type && i.subjectId === subjectId)) return;
    setItems(prev => [...prev, { subjectType: type, subjectId }]);
  }
  function removeItem(idx: number) {
    setItems(prev => prev.filter((_, i) => i !== idx));
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">PLANNED DOWNTIME</div>
        <h1 className="mt-2 text-3xl font-bold">Maintenance Windows</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-400">
          Suppress alerts for chosen devices/OLTs during planned downtime -- a scheduled truck roll or firmware upgrade won&apos;t page anyone. Ported from the previous monitoring setup&apos;s maintenance feature.
        </p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-cyan-900 bg-cyan-950/40 px-4 py-3 text-sm text-cyan-200">{message}</div>}

      {!editing ? (
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">Windows</h2>
            <div className="flex gap-2">
              <button onClick={load} className="rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button>
              <button onClick={() => openEditor(null)} className="rounded-lg bg-cyan-600 px-3 py-2 text-xs font-semibold hover:bg-cyan-500">New maintenance window</button>
            </div>
          </div>
          <div className="mt-5 space-y-3">
            {loading ? (
              <div className="py-8 text-center text-slate-500">Loading…</div>
            ) : windows.length ? (
              windows.map(w => (
                <div key={w.id} className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-950 p-4">
                  <div>
                    <div className="font-medium">{w.title}</div>
                    <div className="mt-1 flex items-center gap-2 text-xs text-slate-500">
                      <span className="rounded bg-slate-800 px-1.5 py-0.5 uppercase">{w.strategy}</span>
                      {w.strategy === "recurring" ? (
                        <span>{(w.daysOfWeek ?? []).map(d => DOW[d]).join(", ") || "no days"} at {w.startTimeOfDay ?? "--"} ({w.timezone}, {w.durationMinutes}m)</span>
                      ) : (
                        <span>{w.startsAt ? new Date(w.startsAt).toLocaleString() : "--"} → {w.endsAt ? new Date(w.endsAt).toLocaleString() : "--"}</span>
                      )}
                      <span className={`rounded-full px-2 py-0.5 ${w.active ? "bg-emerald-950 text-emerald-300" : "bg-slate-800 text-slate-400"}`}>{w.active ? "Active" : "Disabled"}</span>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <button onClick={() => openEditor(w)} className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Edit</button>
                    <button onClick={() => remove(w)} className="rounded-lg border border-red-900 px-3 py-1.5 text-xs text-red-300 hover:bg-red-950/40">Delete</button>
                  </div>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-slate-500">No maintenance windows yet.</div>
            )}
          </div>
        </section>
      ) : (
        <section className={card}>
          <div className="mb-5 flex items-center justify-between">
            <h2 className="font-semibold">{editing.id ? `Edit "${editing.title}"` : "New maintenance window"}</h2>
            <button onClick={() => setEditing(null)} className="text-slate-500 hover:text-white">✕</button>
          </div>
          <form onSubmit={save} className="grid gap-4 sm:grid-cols-2">
            <label className="text-sm text-slate-300">Title<input required name="title" defaultValue={editing.title} className={input} /></label>
            <label className="text-sm text-slate-300">Strategy
              <select name="strategy" defaultValue={editing.strategy} className={input} onChange={() => { /* re-render happens on save; both field sets are always in the form */ }}>
                <option value="single">Single (one-off window)</option>
                <option value="recurring">Recurring (weekly)</option>
              </select>
            </label>
            <label className="sm:col-span-2 text-sm text-slate-300">Description<textarea name="description" defaultValue={editing.description} rows={2} className={input} /></label>

            <div className="sm:col-span-2 grid gap-4 sm:grid-cols-2 rounded-lg border border-slate-800 bg-slate-950 p-4">
              <div className="sm:col-span-2 text-xs font-semibold text-cyan-400">SINGLE WINDOW (used when strategy = single)</div>
              <label className="text-sm text-slate-300">Starts at<input type="datetime-local" name="startsAt" defaultValue={editing.startsAt ? editing.startsAt.slice(0, 16) : ""} className={input} /></label>
              <label className="text-sm text-slate-300">Ends at<input type="datetime-local" name="endsAt" defaultValue={editing.endsAt ? editing.endsAt.slice(0, 16) : ""} className={input} /></label>
            </div>

            <div className="sm:col-span-2 grid gap-4 rounded-lg border border-slate-800 bg-slate-950 p-4">
              <div className="text-xs font-semibold text-cyan-400">RECURRING WINDOW (used when strategy = recurring)</div>
              <div className="flex flex-wrap gap-3 text-sm text-slate-300">
                {DOW.map((d, i) => (
                  <label key={d} className="flex items-center gap-1.5">
                    <input type="checkbox" name={`dow-${i}`} defaultChecked={(editing.daysOfWeek ?? []).includes(i)} className="h-4 w-4" />{d}
                  </label>
                ))}
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <label className="text-sm text-slate-300">Start time of day<input type="time" name="startTimeOfDay" defaultValue={editing.startTimeOfDay ?? "00:00"} className={input} /></label>
                <label className="text-sm text-slate-300">Duration (minutes)<input type="number" min={1} name="durationMinutes" defaultValue={editing.durationMinutes} className={input} /></label>
                <label className="text-sm text-slate-300">Timezone (IANA, e.g. America/New_York)<input name="timezone" defaultValue={editing.timezone} className={`${input} font-mono`} /></label>
              </div>
            </div>

            <label className="flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm"><input name="active" type="checkbox" defaultChecked={editing.active} className="h-4 w-4" /><span>Active (window is honored)</span></label>

            <div className="sm:col-span-2 border-t border-slate-800 pt-4">
              <div className="mb-2 text-xs font-semibold text-cyan-400">DEVICES/OLTS COVERED BY THIS WINDOW</div>
              <div className="flex flex-wrap gap-3">
                <select className={`${input} mt-0 w-auto`} onChange={e => { addItem("device", e.target.value); e.target.value = ""; }}>
                  <option value="">Add a device…</option>
                  {devices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
                </select>
                <select className={`${input} mt-0 w-auto`} onChange={e => { addItem("olt", e.target.value); e.target.value = ""; }}>
                  <option value="">Add an OLT…</option>
                  {olts.map(o => <option key={o.id} value={o.id}>{o.name}</option>)}
                </select>
              </div>
              <div className="mt-3 space-y-2">
                {items.length === 0 && <div className="text-xs text-slate-500">No devices/OLTs added yet -- this window won&apos;t suppress anything.</div>}
                {items.map((it, idx) => (
                  <div key={`${it.subjectType}-${it.subjectId}`} className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm">
                    <span>
                      <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] uppercase text-slate-400">{it.subjectType}</span>
                      <span className="ml-2">{(it.subjectType === "device" ? devices.find(d => d.id === it.subjectId)?.name : olts.find(o => o.id === it.subjectId)?.name) ?? it.subjectId}</span>
                    </span>
                    <button type="button" onClick={() => removeItem(idx)} className="rounded border border-red-900 px-2 py-1 text-xs text-red-300 hover:bg-red-950/40">Remove</button>
                  </div>
                ))}
              </div>
            </div>

            <div className="sm:col-span-2 flex gap-3">
              <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-lg border border-slate-700 px-4 py-3 text-sm">Cancel</button>
              <button disabled={saving} className="flex-1 rounded-lg bg-cyan-600 px-4 py-3 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">{saving ? "Saving…" : "Save maintenance window"}</button>
            </div>
          </form>
        </section>
      )}
    </main>
  );
}
