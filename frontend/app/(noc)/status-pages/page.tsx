"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { apiFetch, ApiError } from "../../../lib/api";

type StatusPage = {
  id: number;
  slug: string;
  title: string;
  description: string;
  published: boolean;
  showCertificateExpiry: boolean;
  footerText: string;
};
type Item = { id?: number; subjectType: "device" | "olt"; subjectId: string; label: string; position: number };
type Device = { id: string; name: string; deviceType: string };
type Olt = { id: string; name: string };

const input = "mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none transition focus:border-cyan-500";
const card = "rounded-2xl border border-slate-800 bg-slate-900 p-5";
const ORG = "tenant-1";

export default function StatusPagesAdmin() {
  const [pages, setPages] = useState<StatusPage[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<StatusPage | null>(null);
  const [items, setItems] = useState<Item[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [olts, setOlts] = useState<Olt[]>([]);
  const [saving, setSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      setPages(await apiFetch<StatusPage[]>("/status-pages"));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load status pages.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);
  useEffect(() => {
    apiFetch<Device[]>(`/devices?organizationId=${ORG}`).then(setDevices).catch(() => {});
    apiFetch<{ id: string; name: string }[]>("/olts").then(setOlts).catch(() => {});
  }, []);

  async function openEditor(p: StatusPage | null) {
    setEditing(p ?? { id: 0, slug: "", title: "", description: "", published: true, showCertificateExpiry: false, footerText: "" });
    if (p) {
      try {
        const full = await apiFetch<StatusPage & { items: Item[] }>(`/status-pages/${p.id}`);
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
    const payload = {
      slug: String(data.get("slug") || "").trim(),
      title: String(data.get("title") || "").trim(),
      description: String(data.get("description") || ""),
      published: data.get("published") === "on",
      showCertificateExpiry: data.get("showCertificateExpiry") === "on",
      footerText: String(data.get("footerText") || ""),
    };
    try {
      let id = editing.id;
      if (id) {
        await apiFetch(`/status-pages/${id}`, { method: "PUT", body: JSON.stringify(payload) });
      } else {
        const created = await apiFetch<StatusPage>("/status-pages", { method: "POST", body: JSON.stringify(payload) });
        id = created.id;
      }
      await apiFetch(`/status-pages/${id}/items`, { method: "PUT", body: JSON.stringify({ items }) });
      setMessage(`Status page "${payload.title}" saved.`);
      setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save status page.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(p: StatusPage) {
    try {
      await apiFetch(`/status-pages/${p.id}`, { method: "DELETE" });
      setMessage(`Status page "${p.title}" deleted.`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete status page.");
    }
  }

  function addItem(type: "device" | "olt", subjectId: string) {
    if (!subjectId) return;
    if (items.some(i => i.subjectType === type && i.subjectId === subjectId)) return;
    setItems(prev => [...prev, { subjectType: type, subjectId, label: "", position: prev.length }]);
  }
  function removeItem(idx: number) {
    setItems(prev => prev.filter((_, i) => i !== idx).map((it, i) => ({ ...it, position: i })));
  }
  function moveItem(idx: number, dir: -1 | 1) {
    setItems(prev => {
      const next = [...prev];
      const target = idx + dir;
      if (target < 0 || target >= next.length) return prev;
      [next[idx], next[target]] = [next[target], next[idx]];
      return next.map((it, i) => ({ ...it, position: i }));
    });
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">CUSTOMER-FACING</div>
        <h1 className="mt-2 text-3xl font-bold">Status Pages</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-400">
          A branded, unauthenticated page showing current status for chosen devices/OLTs -- share the link with customers, or embed it on a support site. Ported from the previous monitoring setup.
        </p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-cyan-900 bg-cyan-950/40 px-4 py-3 text-sm text-cyan-200">{message}</div>}

      {!editing ? (
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">Pages</h2>
            <div className="flex gap-2">
              <button onClick={load} className="rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button>
              <button onClick={() => openEditor(null)} className="rounded-lg bg-cyan-600 px-3 py-2 text-xs font-semibold hover:bg-cyan-500">New status page</button>
            </div>
          </div>
          <div className="mt-5 space-y-3">
            {loading ? (
              <div className="py-8 text-center text-slate-500">Loading…</div>
            ) : pages.length ? (
              pages.map(p => (
                <div key={p.id} className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-950 p-4">
                  <div>
                    <div className="font-medium">{p.title}</div>
                    <div className="mt-1 flex items-center gap-2 text-xs text-slate-500">
                      <a href={`/status/${p.slug}`} target="_blank" rel="noreferrer" className="text-cyan-400 hover:underline">/status/{p.slug}</a>
                      <span className={`rounded-full px-2 py-0.5 ${p.published ? "bg-emerald-950 text-emerald-300" : "bg-slate-800 text-slate-400"}`}>{p.published ? "Published" : "Unpublished"}</span>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <button onClick={() => openEditor(p)} className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Edit</button>
                    <button onClick={() => remove(p)} className="rounded-lg border border-red-900 px-3 py-1.5 text-xs text-red-300 hover:bg-red-950/40">Delete</button>
                  </div>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-slate-500">No status pages yet.</div>
            )}
          </div>
        </section>
      ) : (
        <section className={card}>
          <div className="mb-5 flex items-center justify-between">
            <h2 className="font-semibold">{editing.id ? `Edit "${editing.title}"` : "New status page"}</h2>
            <button onClick={() => setEditing(null)} className="text-slate-500 hover:text-white">✕</button>
          </div>
          <form onSubmit={save} className="grid gap-4 sm:grid-cols-2">
            <label className="text-sm text-slate-300">Title<input required name="title" defaultValue={editing.title} className={input} /></label>
            <label className="text-sm text-slate-300">Slug (in the URL)<input required name="slug" defaultValue={editing.slug} placeholder="network-status" className={`${input} font-mono`} /></label>
            <label className="sm:col-span-2 text-sm text-slate-300">Description<textarea name="description" defaultValue={editing.description} rows={2} className={input} /></label>
            <label className="sm:col-span-2 text-sm text-slate-300">Footer text<input name="footerText" defaultValue={editing.footerText} className={input} /></label>
            <label className="flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm"><input name="published" type="checkbox" defaultChecked={editing.published} className="h-4 w-4" /><span>Published (publicly reachable)</span></label>
            <label className="flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm"><input name="showCertificateExpiry" type="checkbox" defaultChecked={editing.showCertificateExpiry} className="h-4 w-4" /><span>Show TLS cert expiry</span></label>

            <div className="sm:col-span-2 border-t border-slate-800 pt-4">
              <div className="mb-2 text-xs font-semibold text-cyan-400">MONITORS ON THIS PAGE</div>
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
                {items.length === 0 && <div className="text-xs text-slate-500">No monitors added yet.</div>}
                {items.map((it, idx) => (
                  <div key={`${it.subjectType}-${it.subjectId}`} className="flex items-center justify-between rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm">
                    <span>
                      <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] uppercase text-slate-400">{it.subjectType}</span>
                      <span className="ml-2">{(it.subjectType === "device" ? devices.find(d => d.id === it.subjectId)?.name : olts.find(o => o.id === it.subjectId)?.name) ?? it.subjectId}</span>
                    </span>
                    <span className="flex gap-1">
                      <button type="button" onClick={() => moveItem(idx, -1)} className="rounded border border-slate-700 px-2 py-1 text-xs hover:bg-slate-800">↑</button>
                      <button type="button" onClick={() => moveItem(idx, 1)} className="rounded border border-slate-700 px-2 py-1 text-xs hover:bg-slate-800">↓</button>
                      <button type="button" onClick={() => removeItem(idx)} className="rounded border border-red-900 px-2 py-1 text-xs text-red-300 hover:bg-red-950/40">Remove</button>
                    </span>
                  </div>
                ))}
              </div>
            </div>

            <div className="sm:col-span-2 flex gap-3">
              <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-lg border border-slate-700 px-4 py-3 text-sm">Cancel</button>
              <button disabled={saving} className="flex-1 rounded-lg bg-cyan-600 px-4 py-3 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">{saving ? "Saving…" : "Save status page"}</button>
            </div>
          </form>
        </section>
      )}
    </main>
  );
}
