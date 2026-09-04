"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Tag = { id: number; name: string; color: string };

const input = "mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none transition focus:border-cyan-500";
const card = "rounded-2xl border border-slate-800 bg-slate-900 p-5";
const ORG = "tenant-1";
const DEFAULT_COLOR = "#58A6FF";
const SWATCHES = ["#58A6FF", "#3FB950", "#D29922", "#F85149", "#A371F7", "#8B949E", "#DB61A2"];

export default function TagsAdmin() {
  const [tags, setTags] = useState<Tag[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Tag | null>(null);
  const [saving, setSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      setTags(await apiFetch<Tag[]>(`/tags?tenantId=${ORG}`));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load tags.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    setSaving(true);
    setMessage("");
    const data = new FormData(e.currentTarget);
    const payload = { tenantId: ORG, name: String(data.get("name") || "").trim(), color: String(data.get("color") || DEFAULT_COLOR) };
    try {
      if (editing.id) {
        await apiFetch(`/tags/${editing.id}`, { method: "PUT", body: JSON.stringify(payload) });
      } else {
        await apiFetch("/tags", { method: "POST", body: JSON.stringify(payload) });
      }
      setMessage(`Tag "${payload.name}" saved.`);
      setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save tag.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(t: Tag) {
    try {
      await apiFetch(`/tags/${t.id}`, { method: "DELETE" });
      setMessage(`Tag "${t.name}" deleted.`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete tag.");
    }
  }

  return (
    <main className="mx-auto max-w-4xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">ORGANIZE</div>
        <h1 className="mt-2 text-3xl font-bold">Tags</h1>
        <p className="mt-2 max-w-2xl text-sm text-slate-400">
          Free-form, colored labels for devices and OLTs -- ported from the previous monitoring setup. Assign tags from a device&apos;s detail page.
        </p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-cyan-900 bg-cyan-950/40 px-4 py-3 text-sm text-cyan-200">{message}</div>}

      {!editing ? (
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">All tags</h2>
            <button onClick={() => setEditing({ id: 0, name: "", color: DEFAULT_COLOR })} className="rounded-lg bg-cyan-600 px-3 py-2 text-xs font-semibold hover:bg-cyan-500">New tag</button>
          </div>
          <div className="mt-5 flex flex-wrap gap-2">
            {loading ? (
              <div className="py-8 text-center text-slate-500">Loading…</div>
            ) : tags.length ? (
              tags.map(t => (
                <div key={t.id} className="flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium" style={{ borderColor: t.color, backgroundColor: `${t.color}22`, color: t.color }}>
                  <span>{t.name}</span>
                  <button onClick={() => setEditing(t)} className="opacity-70 hover:opacity-100">✎</button>
                  <button onClick={() => remove(t)} className="opacity-70 hover:opacity-100">✕</button>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-slate-500">No tags yet.</div>
            )}
          </div>
        </section>
      ) : (
        <section className={card}>
          <div className="mb-5 flex items-center justify-between">
            <h2 className="font-semibold">{editing.id ? `Edit "${editing.name}"` : "New tag"}</h2>
            <button onClick={() => setEditing(null)} className="text-slate-500 hover:text-white">✕</button>
          </div>
          <form onSubmit={save} className="grid gap-4">
            <label className="text-sm text-slate-300">Name<input required name="name" defaultValue={editing.name} placeholder="core, needs-firmware, customer-edge…" className={input} /></label>
            <div>
              <div className="text-sm text-slate-300">Color</div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {SWATCHES.map(c => (
                  <label key={c} className="h-7 w-7 cursor-pointer rounded-full border-2" style={{ backgroundColor: c, borderColor: editing.color === c ? "#E6EDF3" : "transparent" }}>
                    <input type="radio" name="color" value={c} defaultChecked={editing.color === c} className="sr-only" onChange={() => setEditing(prev => prev ? { ...prev, color: c } : prev)} />
                  </label>
                ))}
              </div>
            </div>
            <div className="flex gap-3">
              <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-lg border border-slate-700 px-4 py-3 text-sm">Cancel</button>
              <button disabled={saving} className="flex-1 rounded-lg bg-cyan-600 px-4 py-3 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">{saving ? "Saving…" : "Save tag"}</button>
            </div>
          </form>
        </section>
      )}
    </main>
  );
}
