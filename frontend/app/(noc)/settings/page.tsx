"use client";

import { FormEvent, useEffect, useState } from "react";
import { apiFetch, ApiError } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/primitives";
import { PageHeader, Banner, Input, FieldLabel, Panel } from "../../../components/ui/form";

type Me = { username: string; mustChangePassword: boolean; twoFAEnabled: boolean };

type ApiKey = {
  id: number;
  name: string;
  active: boolean;
  expiresAt?: string;
  createdDate: string;
};

export default function SettingsPage() {
  const [me, setMe] = useState<Me | null>(null);

  useEffect(() => {
    apiFetch<Me>("/auth/me")
      .then(setMe)
      .catch(() => setMe(null));
  }, []);

  return (
    <main className="mx-auto max-w-4xl px-6 py-6">
      <PageHeader
        eyebrow="Account"
        title="Settings"
        description={me ? `Signed in as ${me.username}. Manage two-factor authentication and API keys for this account.` : "Account security settings."}
      />
      <div className="grid gap-6">
        <TwoFACard />
        <ApiKeysCard />
        <RetentionCard />
        <BackupCard />
      </div>
    </main>
  );
}

const ORG = "tenant-1";

type ImportMode = "skip" | "keep" | "overwrite";

type ImportResult = {
  mode: string;
  devicesImported: number;
  devicesSkipped: number;
  tagsImported: number;
  deviceGroupsImported: number;
  notificationChannelsImported: number;
  notificationChannelsSkipped: number;
  alertRulesImported: number;
  alertRulesSkipped: number;
  warnings?: string[];
};

function BackupCard() {
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [mode, setMode] = useState<ImportMode>("skip");
  const [file, setFile] = useState<File | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [result, setResult] = useState<ImportResult | null>(null);

  async function exportBackup() {
    setExporting(true);
    setError("");
    setMessage("");
    try {
      const res = await fetch(`/api/v1/backup?tenantId=${ORG}`, { credentials: "include", cache: "no-store" });
      if (!res.ok) throw new Error(`Export failed with status ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      const date = new Date().toISOString().slice(0, 10);
      a.href = url;
      a.download = `routingnms-backup-${date}.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      setMessage("Backup exported and downloaded.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to export backup.");
    } finally {
      setExporting(false);
    }
  }

  async function importBackup(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!file) {
      setError("Choose a backup JSON file first.");
      return;
    }
    setImporting(true);
    setError("");
    setMessage("");
    setResult(null);
    try {
      const text = await file.text();
      const res = await apiFetch<ImportResult>(`/backup/import?mode=${mode}&tenantId=${ORG}`, {
        method: "POST",
        body: text,
      });
      setResult(res);
      setMessage("Backup imported successfully.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to import backup — check the file is a valid RoutingNMS backup JSON.");
    } finally {
      setImporting(false);
    }
  }

  return (
    <Card title="Backup & restore" className="p-5">
      <p className="mb-4 text-xs text-[#8B949E]">
        Export or restore your device inventory, tags, device groups, notification channels and alert rules as a single JSON file — configuration
        only, no heartbeat/metric history. SNMP credentials (community strings, v3 auth/privacy passwords) are never included in an export; a
        restored device always comes back with SNMP monitoring disabled and must be reconfigured by hand, the same way the device list&apos;s
        &quot;Clone&quot; button deliberately doesn&apos;t carry SNMP credentials over.
      </p>
      {message && <Banner>{message}</Banner>}
      {error && <Banner tone="error">{error}</Banner>}

      <div className="mb-6 flex items-center justify-between rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4">
        <div>
          <div className="text-sm font-medium text-[#E6EDF3]">Export backup</div>
          <div className="mt-0.5 text-xs text-[#8B949E]">Downloads a JSON snapshot of your current configuration.</div>
        </div>
        <Button variant="primary" disabled={exporting} onClick={exportBackup}>
          {exporting ? "Exporting…" : "Export backup"}
        </Button>
      </div>

      <form onSubmit={importBackup} className="grid gap-3 rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4">
        <div className="text-sm font-medium text-[#E6EDF3]">Import backup</div>
        <FieldLabel>
          Backup JSON file
          <Input
            required
            type="file"
            accept="application/json,.json"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </FieldLabel>
        <FieldLabel>
          Import mode
          <select
            value={mode}
            onChange={(e) => setMode(e.target.value as ImportMode)}
            className="mt-1 w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm text-[#E6EDF3] outline-none focus:border-[#58A6FF]"
          >
            <option value="skip">Skip — only import rows whose name doesn&apos;t already exist</option>
            <option value="keep">Keep — import everything as new rows alongside existing data</option>
            <option value="overwrite">Overwrite — delete existing configuration first, then import (destructive)</option>
          </select>
        </FieldLabel>
        {mode === "overwrite" && (
          <Banner tone="error">
            Overwrite deletes ALL existing devices, tags, device groups, notification channels and alert rules for this tenant before
            importing. This cannot be undone.
          </Banner>
        )}
        <Button variant={mode === "overwrite" ? "danger" : "primary"} disabled={importing} type="submit">
          {importing ? "Importing…" : "Import backup"}
        </Button>
      </form>

      {result && (
        <Panel className="mt-4">
          <div className="text-xs font-semibold text-[#3FB950]">Import complete ({result.mode} mode):</div>
          <ul className="mt-2 grid gap-1 text-xs text-[#C9D1D9] sm:grid-cols-2">
            <li>Devices imported: {result.devicesImported}{result.devicesSkipped ? ` (${result.devicesSkipped} skipped)` : ""}</li>
            <li>Tags imported: {result.tagsImported}</li>
            <li>Device groups imported: {result.deviceGroupsImported}</li>
            <li>
              Notification channels imported: {result.notificationChannelsImported}
              {result.notificationChannelsSkipped ? ` (${result.notificationChannelsSkipped} skipped)` : ""}
            </li>
            <li>
              Alert rules imported: {result.alertRulesImported}
              {result.alertRulesSkipped ? ` (${result.alertRulesSkipped} skipped)` : ""}
            </li>
          </ul>
          {result.warnings?.map((w, i) => (
            <div key={i} className="mt-2 text-xs text-[#D29922]">
              ⚠ {w}
            </div>
          ))}
        </Panel>
      )}
    </Card>
  );
}

type RetentionSetting = { tenantId: string; dataRetentionDays: number };

function RetentionCard() {
  const [days, setDays] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    apiFetch<RetentionSetting>(`/tenants/${ORG}/retention`)
      .then((res) => setDays(String(res.dataRetentionDays)))
      .catch(() => setError("Unable to load the retention setting."))
      .finally(() => setLoading(false));
  }, []);

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setSaving(true);
    setError("");
    setMessage("");
    const value = days.trim() === "" ? 0 : Number(days);
    try {
      await apiFetch<RetentionSetting>(`/tenants/${ORG}/retention`, {
        method: "PUT",
        body: JSON.stringify({ dataRetentionDays: value }),
      });
      setMessage(
        value < 1
          ? "Saved — monitoring data will be kept forever."
          : `Saved — monitoring data older than ${value} day${value === 1 ? "" : "s"} will be deleted automatically.`
      );
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to save the retention setting.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card title="Data retention" className="p-5">
      <p className="mb-4 text-xs text-[#8B949E]">
        How long RoutingNMS keeps monitoring history (ICMP/HTTP/DNS/push/SSH/Telnet/SNMP metric samples used for charts and sparklines) before
        deleting it automatically. A background job runs once a day and permanently removes samples older than this period.
      </p>
      {message && <Banner>{message}</Banner>}
      {error && <Banner tone="error">{error}</Banner>}
      <form onSubmit={save} className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
        <FieldLabel>
          Keep monitoring data for N days (0 or blank = keep forever)
          <Input
            type="number"
            min={0}
            placeholder="180"
            value={days}
            disabled={loading}
            onChange={(e) => setDays(e.target.value)}
            className="w-40"
          />
        </FieldLabel>
        <Button variant="primary" disabled={loading || saving} type="submit">
          {saving ? "Saving…" : "Save"}
        </Button>
      </form>
    </Card>
  );
}

function TwoFACard() {
  const [status, setStatus] = useState<"unknown" | "enabled" | "disabled">("unknown");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // Setup flow state: prepare -> shows secret+URI, waits for a verifying
  // token; disable flow: just needs the current password.
  const [prepared, setPrepared] = useState<{ secret: string; uri: string } | null>(null);
  const [showDisable, setShowDisable] = useState(false);

  useEffect(() => {
    apiFetch<Me>("/auth/me")
      .then((m) => setStatus(m.twoFAEnabled ? "enabled" : "disabled"))
      .catch(() => setStatus("unknown"));
  }, []);

  async function prepare(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    const data = new FormData(e.currentTarget);
    try {
      const res = await apiFetch<{ secret: string; uri: string }>("/auth/2fa/prepare", {
        method: "POST",
        body: JSON.stringify({ password: data.get("password") }),
      });
      setPrepared(res);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start 2FA setup.");
    } finally {
      setBusy(false);
    }
  }

  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    const data = new FormData(e.currentTarget);
    try {
      await apiFetch("/auth/2fa/save", {
        method: "POST",
        body: JSON.stringify({ password: data.get("password"), token: data.get("token") }),
      });
      setStatus("enabled");
      setPrepared(null);
      setMessage("Two-factor authentication is now enabled on your account.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not verify the code. Check your authenticator app and try again.");
    } finally {
      setBusy(false);
    }
  }

  async function disable(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    const data = new FormData(e.currentTarget);
    try {
      await apiFetch("/auth/2fa/disable", {
        method: "POST",
        body: JSON.stringify({ password: data.get("password") }),
      });
      setStatus("disabled");
      setShowDisable(false);
      setMessage("Two-factor authentication has been disabled.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not disable two-factor authentication.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title="Two-factor authentication (TOTP)" className="p-5">
      <p className="mb-4 text-xs text-[#8B949E]">
        Require a time-based one-time code from an authenticator app (Google Authenticator, Authy, 1Password, etc.) in addition to your
        password when logging in.
      </p>
      {message && <Banner>{message}</Banner>}
      {error && <Banner tone="error">{error}</Banner>}

      {status === "enabled" && !showDisable && !prepared && (
        <div className="flex items-center justify-between rounded-[6px] border border-[#2EA043] bg-[#12261E] px-4 py-3">
          <span className="text-sm font-medium text-[#3FB950]">Two-factor authentication is enabled on your account.</span>
          <Button variant="danger" onClick={() => setShowDisable(true)}>
            Disable 2FA
          </Button>
        </div>
      )}

      {status !== "enabled" && !prepared && !showDisable && (
        <form onSubmit={prepare} className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
          <FieldLabel>
            Current password
            <Input required name="password" type="password" placeholder="Confirm it's you before starting setup" />
          </FieldLabel>
          <Button variant="primary" disabled={busy} type="submit">
            {busy ? "Starting…" : "Enable 2FA"}
          </Button>
        </form>
      )}

      {showDisable && !prepared && (
        <form onSubmit={disable} className="grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
          <FieldLabel>
            Current password
            <Input required name="password" type="password" placeholder="Confirm it's you to disable 2FA" />
          </FieldLabel>
          <div className="flex gap-2">
            <Button variant="danger" disabled={busy} type="submit">
              {busy ? "Disabling…" : "Confirm disable"}
            </Button>
            <Button type="button" onClick={() => setShowDisable(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}

      {prepared && (
        <div className="grid gap-4">
          <Panel>
            <div className="text-xs text-[#8B949E]">
              Add this account to your authenticator app using manual entry (no QR scanner is bundled in this build) — enter the key below
              exactly as shown, or paste the full URI if your app accepts one.
            </div>
            <div className="mt-3 grid gap-2">
              <div>
                <div className="label">Secret key</div>
                <div className="mt-1 select-all break-all rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 font-mono text-sm text-[#79C0FF]">
                  {prepared.secret}
                </div>
              </div>
              <div>
                <div className="label">otpauth:// URI</div>
                <div className="mt-1 select-all break-all rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 font-mono text-[11px] text-[#8B949E]">
                  {prepared.uri}
                </div>
              </div>
            </div>
          </Panel>
          <form onSubmit={save} className="grid gap-3 sm:grid-cols-3 sm:items-end">
            <FieldLabel>
              Current password
              <Input required name="password" type="password" />
            </FieldLabel>
            <FieldLabel>
              6-digit code from your app
              <Input required name="token" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} placeholder="123456" />
            </FieldLabel>
            <div className="flex gap-2">
              <Button variant="primary" disabled={busy} type="submit">
                {busy ? "Verifying…" : "Confirm & enable"}
              </Button>
              <Button type="button" onClick={() => setPrepared(null)}>
                Cancel
              </Button>
            </div>
          </form>
        </div>
      )}
    </Card>
  );
}

function ApiKeysCard() {
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [newKey, setNewKey] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    try {
      setKeys(await apiFetch<ApiKey[]>("/auth/api-keys"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Unable to load API keys.");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, []);

  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setCreating(true);
    setError("");
    setMessage("");
    setNewKey(null);
    const data = new FormData(e.currentTarget);
    const expiresDays = String(data.get("expiresDays") || "");
    const expiresAt = expiresDays ? new Date(Date.now() + Number(expiresDays) * 86400000).toISOString() : undefined;
    try {
      const res = await apiFetch<ApiKey & { key: string }>("/auth/api-keys", {
        method: "POST",
        body: JSON.stringify({ name: data.get("name"), expiresAt }),
      });
      setNewKey(res.key);
      e.currentTarget.reset();
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create API key.");
    } finally {
      setCreating(false);
    }
  }

  async function toggleActive(k: ApiKey) {
    try {
      await apiFetch(`/auth/api-keys/${k.id}`, { method: "PUT", body: JSON.stringify({ active: !k.active }) });
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update API key.");
    }
  }

  async function remove(k: ApiKey) {
    try {
      await apiFetch(`/auth/api-keys/${k.id}`, { method: "DELETE" });
      setMessage(`API key "${k.name}" deleted.`);
      await load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete API key.");
    }
  }

  return (
    <Card title="API keys" className="p-5">
      <p className="mb-4 text-xs text-[#8B949E]">
        Issue a bearer token for scripts/integrations. Send it as <code className="text-[#79C0FF]">Authorization: Bearer &lt;key&gt;</code> —
        requests are then authenticated as this account. The raw key is shown once, at creation; only a hash of it is stored.
      </p>
      {message && <Banner>{message}</Banner>}
      {error && <Banner tone="error">{error}</Banner>}

      {newKey && (
        <Panel className="mb-4">
          <div className="text-xs font-semibold text-[#3FB950]">Key created — copy it now, it will not be shown again:</div>
          <div className="mt-2 select-all break-all rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 font-mono text-sm text-[#79C0FF]">
            {newKey}
          </div>
        </Panel>
      )}

      <form onSubmit={create} className="mb-5 grid gap-3 sm:grid-cols-[1fr_auto_auto] sm:items-end">
        <FieldLabel>
          Name
          <Input required name="name" placeholder="ci-monitor-poller" />
        </FieldLabel>
        <FieldLabel>
          Expires in (days, blank = never)
          <Input name="expiresDays" type="number" min={1} placeholder="never" className="w-40" />
        </FieldLabel>
        <Button variant="primary" disabled={creating} type="submit">
          {creating ? "Creating…" : "Create key"}
        </Button>
      </form>

      <div className="overflow-x-auto">
        <table className="tbl w-full min-w-[600px] text-left text-xs">
          <thead>
            <tr>
              <th>Name</th>
              <th>Created</th>
              <th>Expires</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5} className="py-8 text-center text-[#8B949E]">
                  Loading…
                </td>
              </tr>
            ) : keys.length ? (
              keys.map((k) => (
                <tr key={k.id}>
                  <td className="text-[#E6EDF3]">{k.name}</td>
                  <td className="text-[#8B949E]">{new Date(k.createdDate).toLocaleString()}</td>
                  <td className="text-[#8B949E]">{k.expiresAt ? new Date(k.expiresAt).toLocaleString() : "Never"}</td>
                  <td>
                    <span className={k.active ? "text-[#3FB950]" : "text-[#8B949E]"}>{k.active ? "Active" : "Disabled"}</span>
                  </td>
                  <td className="flex gap-2 py-2">
                    <Button onClick={() => toggleActive(k)}>{k.active ? "Disable" : "Enable"}</Button>
                    <Button variant="danger" onClick={() => remove(k)}>
                      Delete
                    </Button>
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td colSpan={5} className="py-8 text-center text-[#8B949E]">
                  No API keys yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
