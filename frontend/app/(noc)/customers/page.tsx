"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";

type Customer = {
  id: number;
  customerName: string;
  customerCode: string;
  accessPointId?: number | null;
  deviceId?: number | null;
  planName?: string;
  ipAddress?: string;
  macAddress?: string;
  bandwidthDlMbps: number;
  bandwidthUlMbps: number;
  isActive: boolean;
  contractStart?: string | null;
  contractEnd?: string | null;
  notes?: string;
  createdAt: string;
  updatedAt: string;
};

const input = "mt-1 w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2.5 text-sm outline-none transition focus:border-[#58A6FF]";
const card = "rounded-[8px] border border-[#21262D] bg-[#161B22] p-5";

function num(v: FormDataEntryValue | null): number {
  if (typeof v !== "string" || v.trim() === "") return 0;
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}
function optNum(v: FormDataEntryValue | null): number | undefined {
  if (typeof v !== "string" || v.trim() === "") return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
}

export default function CustomersPage() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [editing, setEditing] = useState<Customer | null>(null);

  async function load() {
    setLoading(true);
    try {
      setCustomers(await apiFetch<Customer[]>("/customers"));
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load customers.");
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
      customerName: data.get("customerName"),
      customerCode: data.get("customerCode"),
      accessPointId: optNum(data.get("accessPointId")),
      deviceId: optNum(data.get("deviceId")),
      planName: data.get("planName"),
      ipAddress: data.get("ipAddress"),
      macAddress: data.get("macAddress"),
      bandwidthDlMbps: num(data.get("bandwidthDlMbps")),
      bandwidthUlMbps: num(data.get("bandwidthUlMbps")),
      isActive: data.get("isActive") === "on",
      contractStart: (data.get("contractStart") as string) || null,
      contractEnd: (data.get("contractEnd") as string) || null,
      notes: data.get("notes"),
    });
    try {
      if (editing) {
        await apiFetch<Customer>(`/customers/${editing.id}`, { method: "PUT", body });
        setMessage(`Customer "${data.get("customerName")}" updated.`);
      } else {
        await apiFetch<Customer>("/customers", { method: "POST", body });
        setMessage(`Customer "${data.get("customerName")}" created.`);
      }
      setEditing(null);
      e.currentTarget.reset();
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save customer.");
    } finally {
      setSaving(false);
    }
  }

  async function remove(c: Customer) {
    try {
      await apiFetch(`/customers/${c.id}`, { method: "DELETE" });
      setMessage(`Customer "${c.customerName}" deleted.`);
      if (editing?.id === c.id) setEditing(null);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to delete customer.");
    }
  }

  return (
    <main className="mx-auto max-w-7xl px-6 py-8">
      <div className="mb-8">
        <div className="text-xs font-semibold tracking-[.2em] text-[#58A6FF]">ISP</div>
        <h1 className="mt-2 text-3xl font-bold">Customers</h1>
        <p className="mt-2 max-w-3xl text-sm text-[#8B949E]">
          Subscriber connections to your network. Each record links a customer to an access point and (optionally) their
          CPE gateway device, with plan, bandwidth and contract details.
        </p>
      </div>
      {message && <div className="mb-5 rounded-[6px] border border-[#1B4B91] bg-[#11233F] px-4 py-3 text-sm text-[#79C0FF]">{message}</div>}

      <div className="grid gap-6 lg:grid-cols-2">
        <section className={card}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="font-semibold">Customers ({customers.filter(c => c.isActive).length} active)</h2>
            <button onClick={load} className="rounded-[6px] border border-[#30363D] px-3 py-2 text-xs hover:bg-[#21262D]">Refresh</button>
          </div>
          <div className="mt-5 space-y-3">
            {loading ? (
              <div className="py-8 text-center text-[#8B949E]">Loading…</div>
            ) : customers.length ? (
              customers.map(c => (
                <div key={c.id} className="rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4">
                  <div className="flex items-center justify-between">
                    <div className="font-medium">
                      {c.customerName}
                      {!c.isActive && <span className="ml-2 text-xs text-[#8B949E]">(inactive)</span>}
                    </div>
                    <div className="flex gap-2">
                      <button onClick={() => setEditing(c)} className="rounded-[6px] border border-[#30363D] px-3 py-1.5 text-xs hover:bg-[#21262D]">Edit</button>
                      <button onClick={() => remove(c)} className="rounded-[6px] border border-[#672525] px-3 py-1.5 text-xs text-[#F78166] hover:bg-[#2D1212]">Delete</button>
                    </div>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[#8B949E]">
                    <span>Code: {c.customerCode || "—"}</span>
                    <span>Plan: {c.planName || "—"}</span>
                    <span>DL {c.bandwidthDlMbps} / UL {c.bandwidthUlMbps} Mbps</span>
                    {c.ipAddress && <span>IP {c.ipAddress}</span>}
                    {c.accessPointId && <span>AP #{c.accessPointId}</span>}
                  </div>
                </div>
              ))
            ) : (
              <div className="py-8 text-center text-[#8B949E]">No customers yet.</div>
            )}
          </div>
        </section>

        <section className={card}>
          <div className="mb-4">
            <div className="text-xs font-semibold text-[#58A6FF]">{editing ? "EDIT" : "NEW"}</div>
            <h2 className="mt-1 font-semibold">{editing ? `Edit "${editing.customerName}"` : "New customer"}</h2>
          </div>
          <form onSubmit={save} className="grid gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-[#C9D1D9]">
                Name *
                <input required name="customerName" defaultValue={editing?.customerName} className={input} />
              </label>
              <label className="text-sm text-[#C9D1D9]">
                Code *
                <input required name="customerCode" defaultValue={editing?.customerCode} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-[#C9D1D9]">
                Plan
                <input name="planName" defaultValue={editing?.planName} className={input} />
              </label>
              <label className="text-sm text-[#C9D1D9]">
                Access point ID
                <input name="accessPointId" type="number" defaultValue={editing?.accessPointId ?? ""} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-[#C9D1D9]">
                IP address
                <input name="ipAddress" defaultValue={editing?.ipAddress} className={input} />
              </label>
              <label className="text-sm text-[#C9D1D9]">
                MAC address
                <input name="macAddress" defaultValue={editing?.macAddress} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-[#C9D1D9]">
                Download (Mbps)
                <input name="bandwidthDlMbps" type="number" step="any" defaultValue={editing?.bandwidthDlMbps ?? 0} className={input} />
              </label>
              <label className="text-sm text-[#C9D1D9]">
                Upload (Mbps)
                <input name="bandwidthUlMbps" type="number" step="any" defaultValue={editing?.bandwidthUlMbps ?? 0} className={input} />
              </label>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="text-sm text-[#C9D1D9]">
                Contract start
                <input name="contractStart" type="date" defaultValue={editing?.contractStart ?? ""} className={input} />
              </label>
              <label className="text-sm text-[#C9D1D9]">
                Contract end
                <input name="contractEnd" type="date" defaultValue={editing?.contractEnd ?? ""} className={input} />
              </label>
            </div>
            <label className="text-sm text-[#C9D1D9]">
              Notes
              <textarea name="notes" defaultValue={editing?.notes} rows={2} className={input} />
            </label>
            <label className="flex items-center gap-2 text-sm text-[#C9D1D9]">
              <input name="isActive" type="checkbox" defaultChecked={editing ? editing.isActive : true} className="h-4 w-4" />
              Active
            </label>
            <div className="flex gap-3">
              {editing && (
                <button type="button" onClick={() => setEditing(null)} className="flex-1 rounded-[6px] border border-[#30363D] px-4 py-3 text-sm">
                  Cancel
                </button>
              )}
              <button disabled={saving} className="flex-1 rounded-[6px] bg-[#238636] px-4 py-3 text-sm font-semibold hover:bg-[#2EA043] disabled:opacity-50">
                {saving ? "Saving…" : editing ? "Save changes" : "Create customer"}
              </button>
            </div>
          </form>
        </section>
      </div>
    </main>
  );
}
