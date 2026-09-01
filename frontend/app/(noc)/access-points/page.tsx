"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Site = { id: number; name: string; isActive: boolean };
type AccessPoint = {
  id: number;
  name: string;
  siteId?: number | null;
  deviceId?: number | null;
  apType: string;
  frequencyBand?: string;
  channel?: string;
  txPowerDbm?: number | null;
  maxClients?: number | null;
  ipAddress?: string;
  macAddress?: string;
  latitude?: number | null;
  longitude?: number | null;
  monthlyBwLimitGb?: number | null;
  createdAt: string;
  updatedAt: string;
};

const input = "mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2.5 text-sm outline-none transition focus:border-cyan-500";
const card = "rounded-2xl border border-slate-800 bg-slate-900 p-5";
const AP_TYPES = ["sector", "ptp", "ptmp", "olt", "cmts"];

function optNum(v: FormDataEntryValue | null): number | undefined {
  if (typeof v !== "string" || v.trim() === "") return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
}

export default function AccessPointsPage() {
  const [aps, setAps] = useState<AccessPoint[]>([]);
  const [sites, setSites] = useState<Site[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<AccessPoint | null>(null);

  async function load() {
    setLoading(true);
    try {
      const [apList, siteList] = await Promise.all([
        apiFetch<AccessPoint[]>("/access-points"),
        apiFetch<Site[]>("/sites"),
      ]);
      setAps(apList);
      setSites(siteList);
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load access points.");
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
      siteId: optNum(data.get("siteId")),
      deviceId: optNum(data.get("deviceId")),
      apType: data.get("apType"),
      frequencyBand: data.get("frequencyBand"),
      channel: data.get("channel"),
      txPowerDbm: optNum(data.get("txPowerDbm")),
      maxClients: optNum(data.get("maxClients")),
      ipAddress: data.get("ipAddress"),
      macAddress: data.get("macAddress"),
      latitude: optNum(data.get("latitude")),
      longitude: optNum(data.get("longitude")),
      monthlyBwLimitGb: optNum(data.get("monthlyBwLimitGb")),
    });
    try {
      if (editing) {
        await apiFetch<AccessPoint>(`/access-points/${editing.id}`, { method: "PUT", body });
        setMessage(`Access point "${data.get("name")}" updated.`);
      } else {
        await apiFetch<AccessPoint>("/access-points", { method: "POST", body });
        setMessage(`Access point "${data.get("name")}" created.`);
      }
      setEditing(null);
      e.currentTarget.reset();
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save access point.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(ap: AccessPoint) {
    try {
      await apiFetch(`/access-points/${ap.id}`, { method: "DELETE" });
      setMessage(`Access point "${ap.name}" deleted.`);
      if (editing?.id === ap.id) setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete access point.");
    }
  }

  const siteName = (id?: number | null) => sites.find(s => s.id === id)?.name;

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-cyan-400">ISP</div>
        <h1 className="mt-2 text-3xl font-bold">Access Points</h1>
        <p className="mt-2 max-w-3xl text-sm text-slate-400">
          Wireless access points (sector / ptp / ptmp / olt / cmts) linked to a site and, optionally, an SNMP device record.
        </p>
      </div>
      {message && <div className="mb-5 rounded-lg border border-cyan-900 bg-cyan-950/40 px-4 py-3 text-sm text-cyan-200">{message}</div>}

      <div className="grid gap-6 lg:grid-cols-2">
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">Access points</h2>
            <button onClick={load} className="rounded-lg border border-slate-700 px-3 py-2 text-xs hover:bg-slate-800">Refresh</button>
          </div>
          <div className="mt-5 space-y-3">
            {loading ? (
              <div className="py-8 text-center text-slate-500">Loading…</div>
            ) : aps.length ? (
              aps.map(ap => (
                <div key={ap.id} className="rounded-lg border border-slate-800 bg-slate-950 p-4">
                  <div className="flex items-center justify-between">
                    <div className="font-medium">
                      {ap.name} <span className="ml-2 rounded bg-slate-800 px-1.5 py-0.5 text-[10px] uppercase text-cyan-300">{ap.apType}</span>
                    </div>
                    <div className="flex gap-2">
                      <button onClick={() => setEditing(ap)} className="rounded-lg border border-slate-700 px-3 py-1.5 text-xs hover:bg-slate-800">Edit</button>
                      <button onClick={() => remove(ap)} className="rounded-lg border border-red-900 px-3 py-1.5 text-xs text-red-300 hover:bg-red-950/40">Delete</button>
                    </div>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                    <span>Site: {siteName(ap.siteId) || "—"}</span>
                    {ap.frequencyBand && <span>{ap.frequencyBand}</span>}
                    {ap.channel && <span>CH {ap.channel}</span>}
                    {ap.txPowerDbm != null && <span>{ap.txPowerDbm} dBm</span>}
                    {ap.ipAddress && <span>IP {ap.ipAddress}</span>}
                    {ap.maxClients != null && <span>{ap.maxClients} clients</span>}
                  </div>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-slate-500">No access points yet.</div>
            )}
          </div>
        </section>

        <section className={card}>
          <div className="mb-4">
            <div className="text-xs font-semibold text-cyan-400">{editing ? "EDIT" : "NEW"}</div>
            <h2 className="mt-1 font-semibold">{editing ? `Edit "${editing.name}"` : "New access point"}</h2>
          </div>
          <form onSubmit={save} className="grid gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                Name *
                <input required name="name" defaultValue={editing?.name} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                Type
                <select name="apType" defaultValue={editing?.apType || "sector"} className={input}>
                  {AP_TYPES.map(t => <option key={t} value={t}>{t.toUpperCase()}</option>)}
                </select>
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                Site
                <select name="siteId" defaultValue={editing?.siteId ?? ""} className={input}>
                  <option value="">None</option>
                  {sites.filter(s => s.isActive).map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
                </select>
              </label>
              <label className="text-sm text-slate-300">
                Device ID
                <input name="deviceId" type="number" defaultValue={editing?.deviceId ?? ""} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                Frequency band
                <input name="frequencyBand" defaultValue={editing?.frequencyBand} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                Channel
                <input name="channel" defaultValue={editing?.channel} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                TX power (dBm)
                <input name="txPowerDbm" type="number" step="any" defaultValue={editing?.txPowerDbm ?? ""} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                Max clients
                <input name="maxClients" type="number" defaultValue={editing?.maxClients ?? ""} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-slate-300">
                IP address
                <input name="ipAddress" defaultValue={editing?.ipAddress} className={input} />
              </label>
              <label className="text-sm text-slate-300">
                MAC address
                <input name="macAddress" defaultValue={editing?.macAddress} className={input} />
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
              Monthly bandwidth limit (GB)
              <input name="monthlyBwLimitGb" type="number" defaultValue={editing?.monthlyBwLimitGb ?? ""} className={input} />
            </label>
            <div className="flex gap-3">
              {editing && (
                <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-lg border border-slate-700 px-4 py-3 text-sm">
                  Cancel
                </button>
              )}
              <button disabled={saving} className="flex-1 rounded-lg bg-cyan-600 px-4 py-3 text-sm font-semibold hover:bg-cyan-500 disabled:opacity-50">
                {saving ? "Saving…" : editing ? "Save changes" : "Create access point"}
              </button>
            </div>
          </form>
        </section>
      </div>
    </main>
  );
}
