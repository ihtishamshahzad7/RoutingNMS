"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, FieldLabel } from "../../../components/ui/form";

type Tag = { id: number; name: string; color: string };

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
    <main className="mx-auto max-w-4xl px-6 py-6">
      <PageHeader
        eyebrow="Organize"
        title="Tags"
        description="Free-form, colored labels for devices and OLTs — ported from the previous monitoring setup. Assign tags from a device's detail page."
      />
      {message && <Banner>{message}</Banner>}

      {!editing ? (
        <Card
          title={`All tags (${tags.length})`}
          headerRight={<Button variant="primary" onClick={() => setEditing({ id: 0, name: "", color: DEFAULT_COLOR })}>New tag</Button>}
          className="p-4"
        >
          <div className="flex flex-wrap gap-2">
            {loading ? (
              <div className="w-full py-8 text-center text-sm text-[#484F58]">Loading…</div>
            ) : tags.length ? (
              tags.map(t => (
                <div key={t.id} className="flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium" style={{ borderColor: t.color, backgroundColor: `${t.color}22`, color: t.color }}>
                  <span>{t.name}</span>
                  <button onClick={() => setEditing(t)} className="opacity-70 hover:opacity-100">✎</button>
                  <button onClick={() => remove(t)} className="opacity-70 hover:opacity-100">✕</button>
                </div>
              ))
            ) : (
              <div className="w-full py-8 text-center text-sm text-[#484F58]">No tags yet.</div>
            )}
          </div>
        </Card>
      ) : (
        <Card
          title={editing.id ? `Edit "${editing.name}"` : "New tag"}
          headerRight={<button onClick={() => setEditing(null)} className="text-[#8B949E] hover:text-[#E6EDF3]">✕</button>}
          className="p-4"
        >
          <form onSubmit={save} className="grid gap-4">
            <FieldLabel>Name<Input required name="name" defaultValue={editing.name} placeholder="core, needs-firmware, customer-edge…" /></FieldLabel>
            <div>
              <div className="text-sm text-[#8B949E]">Color</div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {SWATCHES.map(c => (
                  <label key={c} className="h-7 w-7 cursor-pointer rounded-full border-2" style={{ backgroundColor: c, borderColor: editing.color === c ? "#E6EDF3" : "transparent" }}>
                    <input type="radio" name="color" value={c} defaultChecked={editing.color === c} className="sr-only" onChange={() => setEditing(prev => prev ? { ...prev, color: c } : prev)} />
                  </label>
                ))}
              </div>
            </div>
            <div className="flex gap-3">
              <Button type="button" onClick={() => setEditing(null)} className="flex-1 justify-center">Cancel</Button>
              <Button variant="primary" disabled={saving} className="flex-1 justify-center">{saving ? "Saving…" : "Save tag"}</Button>
            </div>
          </form>
        </Card>
      )}
    </main>
  );
}
