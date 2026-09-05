"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, Textarea, Select, FieldLabel, Panel, Checkbox } from "../../../components/ui/form";

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
    <main className="mx-auto max-w-7xl px-6 py-6">
      <PageHeader
        eyebrow="Planned downtime"
        title="Maintenance Windows"
        description="Suppress alerts for chosen devices/OLTs during planned downtime — a scheduled truck roll or firmware upgrade won't page anyone. Ported from the previous monitoring setup's maintenance feature."
      />
      {message && <Banner>{message}</Banner>}

      {!editing ? (
        <Card
          title="Windows"
          headerRight={
            <div className="flex gap-2">
              <Button onClick={load}>Refresh</Button>
              <Button variant="primary" onClick={() => openEditor(null)}>New maintenance window</Button>
            </div>
          }
          className="p-4"
        >
          <div className="space-y-3">
            {loading ? (
              <div className="py-8 text-center text-sm text-[#484F58]">Loading…</div>
            ) : windows.length ? (
              windows.map(w => (
                <div key={w.id} className="flex items-center justify-between rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4">
                  <div>
                    <div className="text-sm font-medium text-[#E6EDF3]">{w.title}</div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[#8B949E]">
                      <span className="rounded bg-[#21262D] px-1.5 py-0.5 uppercase">{w.strategy}</span>
                      {w.strategy === "recurring" ? (
                        <span>{(w.daysOfWeek ?? []).map(d => DOW[d]).join(", ") || "no days"} at {w.startTimeOfDay ?? "--"} ({w.timezone}, {w.durationMinutes}m)</span>
                      ) : (
                        <span>{w.startsAt ? new Date(w.startsAt).toLocaleString() : "--"} → {w.endsAt ? new Date(w.endsAt).toLocaleString() : "--"}</span>
                      )}
                      <span className={`rounded-full px-2 py-0.5 ${w.active ? "bg-[#12261E] text-[#3FB950]" : "bg-[#1C2128] text-[#8B949E]"}`}>{w.active ? "Active" : "Disabled"}</span>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button onClick={() => openEditor(w)}>Edit</Button>
                    <Button variant="danger" onClick={() => remove(w)}>Delete</Button>
                  </div>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-sm text-[#484F58]">No maintenance windows yet.</div>
            )}
          </div>
        </Card>
      ) : (
        <Card
          title={editing.id ? `Edit "${editing.title}"` : "New maintenance window"}
          headerRight={<button onClick={() => setEditing(null)} className="text-[#8B949E] hover:text-[#E6EDF3]">✕</button>}
          className="p-4"
        >
          <form onSubmit={save} className="grid gap-4 sm:grid-cols-2">
            <FieldLabel>Title<Input required name="title" defaultValue={editing.title} /></FieldLabel>
            <FieldLabel>Strategy
              <Select name="strategy" defaultValue={editing.strategy}>
                <option value="single">Single (one-off window)</option>
                <option value="recurring">Recurring (weekly)</option>
              </Select>
            </FieldLabel>
            <FieldLabel className="sm:col-span-2">Description<Textarea name="description" defaultValue={editing.description} rows={2} /></FieldLabel>

            <Panel className="sm:col-span-2 grid gap-4 sm:grid-cols-2">
              <div className="sm:col-span-2 label text-[#58A6FF]">Single window (used when strategy = single)</div>
              <FieldLabel>Starts at<Input type="datetime-local" name="startsAt" defaultValue={editing.startsAt ? editing.startsAt.slice(0, 16) : ""} /></FieldLabel>
              <FieldLabel>Ends at<Input type="datetime-local" name="endsAt" defaultValue={editing.endsAt ? editing.endsAt.slice(0, 16) : ""} /></FieldLabel>
            </Panel>

            <Panel className="sm:col-span-2 grid gap-4">
              <div className="label text-[#58A6FF]">Recurring window (used when strategy = recurring)</div>
              <div className="flex flex-wrap gap-3 text-sm text-[#C9D1D9]">
                {DOW.map((d, i) => (
                  <label key={d} className="flex items-center gap-1.5">
                    <Checkbox name={`dow-${i}`} defaultChecked={(editing.daysOfWeek ?? []).includes(i)} />{d}
                  </label>
                ))}
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <FieldLabel>Start time of day<Input type="time" name="startTimeOfDay" defaultValue={editing.startTimeOfDay ?? "00:00"} /></FieldLabel>
                <FieldLabel>Duration (minutes)<Input type="number" min={1} name="durationMinutes" defaultValue={editing.durationMinutes} /></FieldLabel>
                <FieldLabel>Timezone (IANA, e.g. America/New_York)<Input name="timezone" defaultValue={editing.timezone} className="font-mono" /></FieldLabel>
              </div>
            </Panel>

            <label className="flex items-center gap-3 rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3 text-sm text-[#C9D1D9]"><Checkbox name="active" defaultChecked={editing.active} /><span>Active (window is honored)</span></label>

            <div className="sm:col-span-2 border-t border-[#21262D] pt-4">
              <div className="mb-2 label text-[#58A6FF]">Devices/OLTs covered by this window</div>
              <div className="flex flex-wrap gap-3">
                <Select className="w-auto" onChange={e => { addItem("device", e.target.value); e.target.value = ""; }}>
                  <option value="">Add a device…</option>
                  {devices.map(d => <option key={d.id} value={d.id}>{d.name}</option>)}
                </Select>
                <Select className="w-auto" onChange={e => { addItem("olt", e.target.value); e.target.value = ""; }}>
                  <option value="">Add an OLT…</option>
                  {olts.map(o => <option key={o.id} value={o.id}>{o.name}</option>)}
                </Select>
              </div>
              <div className="mt-3 space-y-2">
                {items.length === 0 && <div className="text-xs text-[#484F58]">No devices/OLTs added yet — this window won&apos;t suppress anything.</div>}
                {items.map((it, idx) => (
                  <div key={`${it.subjectType}-${it.subjectId}`} className="flex items-center justify-between rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3 text-sm text-[#C9D1D9]">
                    <span>
                      <span className="rounded bg-[#21262D] px-1.5 py-0.5 text-[10px] uppercase text-[#8B949E]">{it.subjectType}</span>
                      <span className="ml-2">{(it.subjectType === "device" ? devices.find(d => d.id === it.subjectId)?.name : olts.find(o => o.id === it.subjectId)?.name) ?? it.subjectId}</span>
                    </span>
                    <Button type="button" variant="danger" onClick={() => removeItem(idx)}>Remove</Button>
                  </div>
                ))}
              </div>
            </div>

            <div className="sm:col-span-2 flex gap-3">
              <Button type="button" onClick={() => setEditing(null)} className="flex-1 justify-center">Cancel</Button>
              <Button variant="primary" disabled={saving} className="flex-1 justify-center">{saving ? "Saving…" : "Save maintenance window"}</Button>
            </div>
          </form>
        </Card>
      )}
    </main>
  );
}
