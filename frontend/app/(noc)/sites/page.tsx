"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Site = {
  id: number;
  tenantId?: string;
  name: string;
  code?: string;
  address?: string;
  city?: string;
  country?: string;
  latitude?: number | null;
  longitude?: number | null;
  timezone?: string;
  isActive: boolean;
  notes?: string;
  createdAt: string;
  updatedAt: string;
};

const input = "mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none transition focus:border-cyan-500";
const card = "rounded-2xl border border-slate-800 bg-slate-900 p-5";

function num(v: FormDataEntryValue | null): number | undefined {
  if (typeof v !== "string" || v.trim() === "") return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
}

export default function SitesPage() {
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Site | null>(null);

  async function load() {
    setLoading(true);
    try {
      setSites(await apiFetch<Site[]>("/sites"));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load sites.");
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
    const body = JSON.stringify({
      name: data.get("name"),
      code: data.get("code"),
      address: data.get("address"),
      city: data.get("city"),
      country: data.get("country"),
      timezone: data.get("timezone") || "UTC",
      latitude: num(data.get("latitude")),
      longitude: num(data.get("longitude")),
      notes: data.get("notes"),
      isActive: data.get("isActive") === "on",
    });
    try {
      if (editing) {
        await apiFetch<Site>(`/sites/${editing.id}`, { method: "PUT", body });
        setMessage(`Site "${data.get("name")}" updated.`);
      } else {
        await apiFetch<Site>("/sites", { method: "POST", body });
        setMessage(`Site "${data.get("name")}" created.`);
      }
      setEditing(null);
      e.currentTarget.reset();
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save site.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(s: Site) {
    try {
      await apiFetch(`/sites/${s.id}`, { method: "DELETE" });
      setMessage(`Site "${s.name}" deleted.`);
      if (editing?.id === s.id) setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete site.");
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">ISP</div>
        <h1 className="mt-2 text-3xl font-bold">Sites</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-400">
          Physical branch / site locations where you place access points and serve customers.
        </p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-cyan-900 bg-cyan-950/40 px-4 py-3 text-sm text-cyan-200">{message}</div>}

      <div className="grid gap-6 lg:grid-cols-2">
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">Sites</h2>
            <button onClick={load} className="rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button>
          </div>
          <div className="mt-5 space-y-3">
            {loading ? (
              <div className="py-8 text-center text-slate-500">Loading…</div>
            ) : sites.length ? (
              sites.map(s => (
                <div key={s.id} className="rounded-lg border border-slate-800 bg-slate-950 p-4">
                  <div className="flex items-center justify-between">
                    <div className="font-medium">
                      {s.name}
                      {!s.isActive && <span className="ml-2 text-xs text-slate-500">(inactive)</span>}
                    </div>
                    <div className="flex gap-2">
                      <button onClick={() => setEditing(s)} className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Edit</button>
                      <button onClick={() => remove(s)} className="rounded-lg border border-red-900 px-3 py-1.5 text-xs text-red-300 hover:bg-red-950/40">Delete</button>
                    </div>
                  </div>
                  <div className="mt-1 text-xs text-slate-500">
                    {[s.code, s.city, s.country].filter(Boolean).join(" · ") || "No location details"}
                  </div>
                  {s.notes && <div className="mt-1 text-xs text-slate-500">{s.notes}</div>}
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-slate-500">No sites yet.</div>
            )}
          </div>
        </section>

        <section className={card}>
          <div className="mb-4">
            <div className="text-xs font-semibold text-cyan-400">{editing ? "EDIT" : "NEW"}</div>
            <h2 className="mt-1 font-semibold">{editing ? `Edit "${editing.name}"` : "New site"}</h2>
          </div>
          <form onSubmit={save} className="grid gap-4">
            <label className="text-sm text-slate-300">
              Name *
              <input required name="name" defaultValue={editing?.name} className={input} />
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                Code
                <input name="code" defaultValue={editing?.code} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                Timezone
                <input name="timezone" defaultValue={editing?.timezone || "UTC"} className={input} />
              </label>
            </div>
            <label className="text-sm text-slate-300">
              Address
              <input name="address" defaultValue={editing?.address} className={input} />
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                City
                <input name="city" defaultValue={editing?.city} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                Country
                <input name="country" defaultValue={editing?.country} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                Latitude
                <input name="latitude" type="number" step="any" defaultValue={editing?.latitude ?? ""} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                Longitude
                <input name="longitude" type="number" step="any" defaultValue={editing?.longitude ?? ""} className={input} />
              </label>
            </div>
            <label className="text-sm text-slate-300">
              Notes
              <textarea name="notes" defaultValue={editing?.notes} rows={2} className={input} />
            </label>
            <label className="flex items-center gap-2 text-sm text-slate-300">
              <input name="isActive" type="checkbox" defaultChecked={editing ? editing.isActive : true} className="h-4 w-4" />
              Active
            </label>
            <div className="flex gap-3">
              {editing && (
                <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-lg border border-slate-700 px-4 py-3 text-sm">
                  Cancel
                </button>
              )}
              <button disabled={saving} className="flex-1 rounded-lg bg-cyan-600 px-4 py-3 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">
                {saving ? "Saving…" : editing ? "Save changes" : "Create site"}
              </button>
            </div>
          </form>
        </section>
      </div>
    </main>
  );
}
