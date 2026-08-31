"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Template = { id: number; name: string; scriptBody: string; createdAt: string; updatedAt: string };

const input = "mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none transition focus:border-cyan-500";
const card = "rounded-2xl border border-slate-800 bg-slate-900 p-5";

export default function ProvisioningPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Template | null>(null);

  async function load() {
    setLoading(true);
    try {
      setTemplates(await apiFetch<Template[]>("/provisioning/templates"));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load provisioning templates.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setMessage("");
    const data = new FormData(e.currentTarget);
    const body = JSON.stringify({ name: data.get("name"), scriptBody: data.get("scriptBody") });
    try {
      if (editing) {
        await apiFetch<Template>(`/provisioning/templates/${editing.id}`, { method: "PUT", body });
        setMessage(`Template "${data.get("name")}" updated.`);
      } else {
        await apiFetch<Template>("/provisioning/templates", { method: "POST", body });
        setMessage(`Template "${data.get("name")}" created.`);
      }
      setEditing(null);
      e.currentTarget.reset();
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save template.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(t: Template) {
    try {
      await apiFetch(`/provisioning/templates/${t.id}`, { method: "DELETE" });
      setMessage(`Template "${t.name}" deleted.`);
      if (editing?.id === t.id) setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete template.");
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">AUTOMATION</div>
        <h1 className="mt-2 text-3xl font-bold">RouterOS Provisioning Templates</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-400">
          Templates are RouterOS scripts rendered per-device with <code>{"{{.Hostname}}"}</code>, <code>{"{{.Address}}"}</code> and <code>{"{{.Password}}"}</code>.
          Assign a template to a router device from its detail page -- the router then pulls its config itself via <code>/tool fetch</code>.
          Admin must pre-register the router (with its serial number) first; there is no auto-registration of unknown devices.
        </p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-cyan-900 bg-cyan-950/40 px-4 py-3 text-sm text-cyan-200">{message}</div>}

      <div className="grid gap-6 lg:grid-cols-2">
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">Templates</h2>
            <button onClick={load} className="rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button>
          </div>
          <div className="mt-5 space-y-3">
            {loading ? (
              <div className="py-8 text-center text-slate-500">Loading…</div>
            ) : templates.length ? (
              templates.map(t => (
                <div key={t.id} className="rounded-lg border border-slate-800 bg-slate-950 p-4">
                  <div className="flex items-center justify-between">
                    <div className="font-medium">{t.name}</div>
                    <div className="flex gap-2">
                      <button onClick={() => setEditing(t)} className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Edit</button>
                      <button onClick={() => remove(t)} className="rounded-lg border border-red-900 px-3 py-1.5 text-xs text-red-300 hover:bg-red-950/40">Delete</button>
                    </div>
                  </div>
                  <pre className="mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap text-xs text-slate-500">{t.scriptBody}</pre>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-slate-500">No templates yet.</div>
            )}
          </div>
        </section>

        <section className={card}>
          <div className="mb-4">
            <div className="text-xs font-semibold text-cyan-400">{editing ? "EDIT" : "NEW"}</div>
            <h2 className="mt-1 font-semibold">{editing ? `Edit "${editing.name}"` : "New template"}</h2>
          </div>
          <form onSubmit={save} className="grid gap-4">
            <label className="text-sm text-slate-300">
              Name
              <input required name="name" defaultValue={editing?.name} className={input} />
            </label>
            <label className="text-sm text-slate-300">
              Script body
              <textarea required name="scriptBody" defaultValue={editing?.scriptBody} rows={12} className={`${input} font-mono`} />
            </label>
            <div className="flex gap-3">
              {editing && (
                <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-lg border border-slate-700 px-4 py-3 text-sm">
                  Cancel
                </button>
              )}
              <button disabled={saving} className="flex-1 rounded-lg bg-cyan-600 px-4 py-3 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">
                {saving ? "Saving…" : editing ? "Save changes" : "Create template"}
              </button>
            </div>
          </form>
        </section>
      </div>
    </main>
  );
}
