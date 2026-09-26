"use client";

// Feature 1.2 (Device Auto-Discovery), Phase 1 of the RoutingNMS build
// blueprint: manages discovery_targets (saved subnets the backend
// AutoScanner rescans on its own schedule) and reviews the persisted
// discovery_candidates each scan turns up. Distinct from the pre-existing
// one-off "Discover subnet" panel on the Devices page (backend/internal/
// discovery's original Manager/Job flow) -- that flow is unchanged and
// still useful for an ad hoc scan; this page is for a subnet you want
// watched continuously without pressing a button every time.

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, FieldLabel } from "../../../components/ui/form";
import { StatusPill } from "../../../components/ui/status-pill";

const ORG = "tenant-1";

type Target = {
  id: number;
  organizationId: string;
  cidr: string;
  snmpPort: number;
  timeoutMs: number;
  intervalSeconds: number;
  enabled: boolean;
  lastScannedAt?: string;
  lastScanError?: string;
};

type Candidate = {
  id: number;
  targetId: number;
  address: string;
  systemName?: string;
  deviceType: string;
  vendor?: string;
  status: "new" | "imported" | "ignored";
  firstSeenAt: string;
  lastSeenAt: string;
};

export default function AutoDiscoveryPage() {
  const [targets, setTargets] = useState<Target[]>([]);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [adding, setAdding] = useState(false);
  const [saving, setSaving] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const [t, c] = await Promise.all([
        apiFetch<Target[]>(`/discovery/targets?organizationId=${ORG}`),
        apiFetch<Candidate[]>(`/discovery/candidates?organizationId=${ORG}&status=new`),
      ]);
      setTargets(t);
      setCandidates(c);
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : "Unable to load auto-discovery data.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => { load(); }, []);

  async function addTarget(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setMessage("");
    const data = new FormData(e.currentTarget);
    const payload = {
      organizationId: ORG,
      cidr: String(data.get("cidr") || "").trim(),
      version: String(data.get("version") || "2c"),
      community: String(data.get("community") || "public"),
      port: Number(data.get("port") || 161),
      intervalSeconds: Number(data.get("intervalSeconds") || 3600),
    };
    try {
      await apiFetch("/discovery/targets", { method: "POST", body: JSON.stringify(payload) });
      setMessage(`Watching ${payload.cidr} — first scan runs within a minute.`);
      setAdding(false);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to save subnet.");
    } finally {
      setSaving(false);
    }
  }

  async function toggleTarget(t: Target) {
    try {
      await apiFetch(`/discovery/targets/${t.id}`, { method: "PUT", body: JSON.stringify({ enabled: !t.enabled }) });
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to update subnet.");
    }
  }

  async function removeTarget(t: Target) {
    try {
      await apiFetch(`/discovery/targets/${t.id}`, { method: "DELETE" });
      setMessage(`Stopped watching ${t.cidr}.`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Failed to remove subnet.");
    }
  }

  async function reviewCandidate(c: Candidate, action: "import" | "ignore") {
    try {
      await apiFetch(`/discovery/candidates/${c.id}/${action}`, { method: "POST" });
      setMessage(action === "import" ? `Added ${c.address} as a monitored device.` : `Dismissed ${c.address}.`);
      await load();
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : `Failed to ${action} ${c.address}.`);
    }
  }

  return (
    <main className="mx-auto max-w-5xl px-6 py-6">
      <PageHeader
        eyebrow="Network"
        title="Auto-Discovery"
        description="Subnets watched on a schedule for newly-appeared devices — the one-off 'Discover subnet' panel on the Devices page still works for an ad hoc scan; this is for continuous background scanning."
      />
      {message && <Banner>{message}</Banner>}

      <Card
        title={`Watched subnets (${targets.length})`}
        headerRight={<Button variant="primary" onClick={() => setAdding(true)}>Add subnet</Button>}
        className="mb-6 p-4"
      >
        {adding && (
          <form onSubmit={addTarget} className="mb-4 grid grid-cols-2 gap-3 rounded-[6px] border border-border p-3">
            <FieldLabel>CIDR<Input required name="cidr" placeholder="10.0.0.0/24" /></FieldLabel>
            <FieldLabel>Rescan interval (seconds)<Input name="intervalSeconds" type="number" defaultValue={3600} /></FieldLabel>
            <FieldLabel>SNMP version<Input name="version" defaultValue="2c" /></FieldLabel>
            <FieldLabel>SNMP community<Input name="community" defaultValue="public" /></FieldLabel>
            <FieldLabel>SNMP port<Input name="port" type="number" defaultValue={161} /></FieldLabel>
            <div className="col-span-2 flex gap-3">
              <Button type="button" onClick={() => setAdding(false)} className="flex-1 justify-center">Cancel</Button>
              <Button variant="primary" disabled={saving} className="flex-1 justify-center">{saving ? "Saving…" : "Start watching"}</Button>
            </div>
          </form>
        )}
        <div className="flex flex-col gap-2">
          {loading ? (
            <div className="w-full py-8 text-center text-sm text-text-muted">Loading…</div>
          ) : targets.length ? (
            targets.map((t) => (
              <div key={t.id} className="flex items-center justify-between rounded-[6px] border border-border bg-bg-page px-3 py-2 text-sm">
                <div>
                  <span className="font-mono text-text-primary">{t.cidr}</span>
                  <span className="ml-2 text-xs text-text-muted">
                    every {t.intervalSeconds}s
                    {t.lastScannedAt ? ` · last scan ${new Date(t.lastScannedAt).toLocaleString()}` : " · never scanned yet"}
                    {t.lastScanError ? ` · error: ${t.lastScanError}` : ""}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <StatusPill status={t.enabled ? "enabled" : "disabled"} />
                  <button onClick={() => toggleTarget(t)} className="text-xs text-[#58A6FF] hover:underline">
                    {t.enabled ? "Pause" : "Resume"}
                  </button>
                  <button onClick={() => removeTarget(t)} className="text-xs text-[#F78166] hover:underline">Remove</button>
                </div>
              </div>
            ))
          ) : (
            <div className="w-full py-8 text-center text-sm text-text-muted">No subnets watched yet — add one above.</div>
          )}
        </div>
      </Card>

      <Card title={`New devices found (${candidates.length})`} className="p-4">
        <div className="flex flex-col gap-2">
          {candidates.length ? (
            candidates.map((c) => (
              <div key={c.id} className="flex items-center justify-between rounded-[6px] border border-border bg-bg-page px-3 py-2 text-sm">
                <div>
                  <span className="font-mono text-text-primary">{c.address}</span>
                  <span className="ml-2 text-text-muted">{c.systemName || "(no system name)"}</span>
                  <span className="ml-2 text-xs text-text-muted">{c.deviceType}{c.vendor ? ` · ${c.vendor}` : ""}</span>
                  <span className="ml-2 text-xs text-text-muted">first seen {new Date(c.firstSeenAt).toLocaleString()}</span>
                </div>
                <div className="flex items-center gap-3">
                  <button onClick={() => reviewCandidate(c, "import")} className="text-xs text-[#58A6FF] hover:underline">Add as device</button>
                  <button onClick={() => reviewCandidate(c, "ignore")} className="text-xs text-text-muted hover:underline">Dismiss</button>
                </div>
              </div>
            ))
          ) : (
            <div className="w-full py-8 text-center text-sm text-text-muted">No new devices found by watched subnets yet.</div>
          )}
        </div>
      </Card>
    </main>
  );
}
