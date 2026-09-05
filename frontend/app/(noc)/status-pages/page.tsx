"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, Textarea, Select, FieldLabel, Checkbox } from "../../../components/ui/form";

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
    <main className="mx-auto max-w-7xl px-6 py-6">
      <PageHeader
        eyebrow="Customer-facing"
        title="Status Pages"
        description="A branded, unauthenticated page showing current status for chosen devices/OLTs — share the link with customers, or embed it on a support site. Ported from the previous monitoring setup."
      />
      {message && <Banner>{message}</Banner>}

      {!editing ? (
        <Card
          title="Pages"
          headerRight={
            <div className="flex gap-2">
              <Button onClick={load}>Refresh</Button>
              <Button variant="primary" onClick={() => openEditor(null)}>New status page</Button>
            </div>
          }
          className="p-4"
        >
          <div className="space-y-3">
            {loading ? (
              <div className="py-8 text-center text-sm text-[#484F58]">Loading…</div>
            ) : pages.length ? (
              pages.map(p => (
                <div key={p.id} className="flex items-center justify-between rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4">
                  <div>
                    <div className="text-sm font-medium text-[#E6EDF3]">{p.title}</div>
                    <div className="mt-1 flex items-center gap-2 text-xs text-[#8B949E]">
                      <a href={`/status/${p.slug}`} target="_blank" rel="noreferrer" className="text-[#58A6FF] hover:underline">/status/{p.slug}</a>
                      <span className={`rounded-full px-2 py-0.5 ${p.published ? "bg-[#12261E] text-[#3FB950]" : "bg-[#1C2128] text-[#8B949E]"}`}>{p.published ? "Published" : "Unpublished"}</span>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button onClick={() => openEditor(p)}>Edit</Button>
                    <Button variant="danger" onClick={() => remove(p)}>Delete</Button>
                  </div>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-sm text-[#484F58]">No status pages yet.</div>
            )}
          </div>
        </Card>
      ) : (
        <Card
          title={editing.id ? `Edit "${editing.title}"` : "New status page"}
          headerRight={<button onClick={() => setEditing(null)} className="text-[#8B949E] hover:text-[#E6EDF3]">✕</button>}
          className="p-4"
        >
          <form onSubmit={save} className="grid gap-4 sm:grid-cols-2">
            <FieldLabel>Title<Input required name="title" defaultValue={editing.title} /></FieldLabel>
            <FieldLabel>Slug (in the URL)<Input required name="slug" defaultValue={editing.slug} placeholder="network-status" className="font-mono" /></FieldLabel>
            <FieldLabel className="sm:col-span-2">Description<Textarea name="description" defaultValue={editing.description} rows={2} /></FieldLabel>
            <FieldLabel className="sm:col-span-2">Footer text<Input name="footerText" defaultValue={editing.footerText} /></FieldLabel>
            <label className="flex items-center gap-3 rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3 text-sm text-[#C9D1D9]"><Checkbox name="published" defaultChecked={editing.published} /><span>Published (publicly reachable)</span></label>
            <label className="flex items-center gap-3 rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3 text-sm text-[#C9D1D9]"><Checkbox name="showCertificateExpiry" defaultChecked={editing.showCertificateExpiry} /><span>Show TLS cert expiry</span></label>

            <div className="sm:col-span-2 border-t border-[#21262D] pt-4">
              <div className="mb-2 label text-[#58A6FF]">Monitors on this page</div>
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
                {items.length === 0 && <div className="text-xs text-[#484F58]">No monitors added yet.</div>}
                {items.map((it, idx) => (
                  <div key={`${it.subjectType}-${it.subjectId}`} className="flex items-center justify-between rounded-[6px] border border-[#21262D] bg-[#0D1117] p-3 text-sm text-[#C9D1D9]">
                    <span>
                      <span className="rounded bg-[#21262D] px-1.5 py-0.5 text-[10px] uppercase text-[#8B949E]">{it.subjectType}</span>
                      <span className="ml-2">{(it.subjectType === "device" ? devices.find(d => d.id === it.subjectId)?.name : olts.find(o => o.id === it.subjectId)?.name) ?? it.subjectId}</span>
                    </span>
                    <span className="flex gap-1">
                      <Button type="button" onClick={() => moveItem(idx, -1)}>↑</Button>
                      <Button type="button" onClick={() => moveItem(idx, 1)}>↓</Button>
                      <Button type="button" variant="danger" onClick={() => removeItem(idx)}>Remove</Button>
                    </span>
                  </div>
                ))}
              </div>
            </div>

            <div className="sm:col-span-2 flex gap-3">
              <Button type="button" onClick={() => setEditing(null)} className="flex-1 justify-center">Cancel</Button>
              <Button variant="primary" disabled={saving} className="flex-1 justify-center">{saving ? "Saving…" : "Save status page"}</Button>
            </div>
          </form>
        </Card>
      )}
    </main>
  );
}
