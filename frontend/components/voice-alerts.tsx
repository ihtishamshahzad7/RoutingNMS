"use client";

import { useEffect, useRef, useState } from "react";
import { apiFetch } from "../lib/api";
import {
  AlertSeverity,
  AlertSource,
  DEFAULT_VOICE_ALERT_SETTINGS,
  VoiceAlertSettings,
  loadVoiceAlertSettings,
  saveVoiceAlertSettings,
} from "../lib/voice-alert-settings";

type ActiveAlert = {
  id: string;
  source: AlertSource;
  severity: AlertSeverity;
  hostname: string;
  message: string;
  since: string;
  // "down" (default, may be omitted by older backends) for a still-ongoing
  // problem, "up" for a just-recovered one (an OLT alert that cleared, or
  // a device that came back reachable).
  kind?: "down" | "up";
};

const POLL_MS = 15000;
const SOURCE_LABEL: Record<AlertSource, string> = { olt: "OLT alert", device: "Device", trap: "SNMP trap", http: "HTTP check" };

function speechText(a: ActiveAlert): string {
  if (a.kind === "up") {
    if (a.source === "device") return `${a.hostname} is back up.`;
    if (a.source === "olt") return `Alert cleared on ${a.hostname}. ${a.message}.`;
    if (a.source === "http") return `HTTP check on ${a.hostname} recovered.`;
    return `Recovered: ${a.hostname}. ${a.message}.`;
  }
  if (a.source === "device") return `${a.severity}. ${a.hostname} is down.`;
  if (a.source === "olt") return `${a.severity} alert on ${a.hostname}. ${a.message}.`;
  if (a.source === "http") return `${a.severity}. ${a.hostname}. ${a.message}.`;
  return `${a.severity} SNMP trap from ${a.hostname}. ${a.message}.`;
}

/**
 * Mounted once in the (noc) layout, so it's present for every authenticated
 * browser session -- it polls the unified active-alerts feed and speaks new
 * or still-active (past the repeat interval) alerts aloud via the Web
 * Speech API, saying what's down (and, separately, what just came back up)
 * and its hostname. A floating control lets an operator mute it, tune the
 * repeat interval/volume, filter by alert type (OLT/device/SNMP trap) and
 * severity, toggle recovery ("back up") announcements, and mute individual
 * hosts.
 */
export default function VoiceAlerts() {
  const [settings, setSettings] = useState<VoiceAlertSettings>(DEFAULT_VOICE_ALERT_SETTINGS);
  const [alerts, setAlerts] = useState<ActiveAlert[]>([]);
  const [panelOpen, setPanelOpen] = useState(false);
  const [voiceReady, setVoiceReady] = useState(false);
  const lastSpokenRef = useRef<Map<string, number>>(new Map());
  const settingsRef = useRef(settings);
  settingsRef.current = settings;

  useEffect(() => { setSettings(loadVoiceAlertSettings()); }, []);

  useEffect(() => {
    let active = true;
    async function poll() {
      try {
        const result = await apiFetch<ActiveAlert[]>("/alerts/active");
        if (active) setAlerts(result);
      } catch {
        // a transient poll failure shouldn't spam anything -- just try again next tick
      }
    }
    poll();
    const timer = window.setInterval(poll, POLL_MS);
    return () => { active = false; window.clearInterval(timer); };
  }, []);

  useEffect(() => {
    // Deliberately does NOT gate on voiceReady: the Web Speech API's
    // speechSynthesis.speak() isn't covered by the strict media-autoplay
    // block most browsers apply to <audio>/<video> (it's not an
    // HTMLMediaElement), so it works without a prior click in most
    // browsers -- but a couple of browser/version combinations do still
    // require one, which is what the "Test voice" button and its hint are
    // for. Speaking is attempted either way so the common case (works
    // immediately, every session) isn't held hostage by the exception.
    if (!settings.enabled || typeof window === "undefined" || !("speechSynthesis" in window)) return;
    const now = Date.now();
    const s = settingsRef.current;
    for (const a of alerts) {
      if (!s.sources[a.source]) continue;
      if (a.kind === "up") { if (!s.announceRecoveries) continue; }
      else if (!s.severities[a.severity]) continue;
      if (s.mutedHostnames.includes(a.hostname)) continue;
      const last = lastSpokenRef.current.get(a.id) ?? 0;
      if (now - last < s.repeatIntervalSeconds * 1000) continue;
      const utterance = new SpeechSynthesisUtterance(speechText(a));
      utterance.volume = s.volume;
      utterance.onstart = () => setVoiceReady(true);
      window.speechSynthesis.speak(utterance);
      lastSpokenRef.current.set(a.id, now);
    }
    // Forget alerts that are no longer active, so if they recur later they're announced immediately.
    const activeIds = new Set(alerts.map(a => a.id));
    for (const id of Array.from(lastSpokenRef.current.keys())) {
      if (!activeIds.has(id)) lastSpokenRef.current.delete(id);
    }
  }, [alerts, settings.enabled]);

  function update(patch: Partial<VoiceAlertSettings>) {
    const next = { ...settings, ...patch };
    setSettings(next);
    saveVoiceAlertSettings(next);
  }
  function toggleSource(src: AlertSource) { update({ sources: { ...settings.sources, [src]: !settings.sources[src] } }); }
  function toggleSeverity(sev: AlertSeverity) { update({ severities: { ...settings.severities, [sev]: !settings.severities[sev] } }); }
  function toggleMuteHost(host: string) {
    const muted = settings.mutedHostnames.includes(host)
      ? settings.mutedHostnames.filter(h => h !== host)
      : [...settings.mutedHostnames, host];
    update({ mutedHostnames: muted });
  }
  function testVoice() {
    setVoiceReady(true);
    if (typeof window !== "undefined" && "speechSynthesis" in window) {
      const u = new SpeechSynthesisUtterance("Voice alerts are enabled. This is a test.");
      u.volume = settings.volume;
      window.speechSynthesis.speak(u);
    }
  }

  const hostsSeen = Array.from(new Set(alerts.map(a => a.hostname))).sort();
  // Only still-down alerts drive the urgent (red) badge -- a recovery
  // shouldn't make the bell look like there's an active problem.
  const downCount = alerts.filter(a => a.kind !== "up" && settings.sources[a.source] && settings.severities[a.severity] && !settings.mutedHostnames.includes(a.hostname)).length;

  return (
    <div className="fixed bottom-5 right-5 z-40 flex flex-col items-end gap-2">
      {panelOpen && (
        <div className="w-80 rounded-xl border border-slate-700 bg-slate-900 p-4 text-sm shadow-2xl">
          <div className="mb-3 flex items-center justify-between">
            <span className="font-semibold text-slate-200">Voice alerts</span>
            <button onClick={() => setPanelOpen(false)} className="text-slate-500 hover:text-white">✕</button>
          </div>

          {!voiceReady && (
            <div className="mb-3 rounded-lg border border-amber-900 bg-amber-950/40 px-3 py-2 text-xs text-amber-300">
              Browsers require a click before audio can play. Click "Test voice" once per session to enable it.
            </div>
          )}
          <button onClick={testVoice} className="mb-4 w-full rounded-lg border border-cyan-800 bg-cyan-950/40 px-3 py-2 text-xs text-cyan-300 hover:bg-cyan-900/40">
            🔊 Test voice
          </button>

          <label className="mb-3 flex items-center justify-between text-xs text-slate-300">
            <span>Enabled</span>
            <input type="checkbox" checked={settings.enabled} onChange={e => update({ enabled: e.target.checked })} className="h-4 w-4" />
          </label>

          <label className="mb-3 flex items-center justify-between text-xs text-slate-300">
            <span>Announce recoveries ("back up")</span>
            <input type="checkbox" checked={settings.announceRecoveries} onChange={e => update({ announceRecoveries: e.target.checked })} className="h-4 w-4" />
          </label>

          <label className="mb-3 block text-xs text-slate-300">
            Repeat interval (seconds)
            <input type="number" min={15} value={settings.repeatIntervalSeconds} onChange={e => update({ repeatIntervalSeconds: Math.max(15, Number(e.target.value) || 15) })} className="mt-1 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm" />
          </label>

          <label className="mb-4 block text-xs text-slate-300">
            Volume
            <input type="range" min={0} max={1} step={0.1} value={settings.volume} onChange={e => update({ volume: Number(e.target.value) })} className="mt-1 w-full" />
          </label>

          <div className="mb-4">
            <div className="mb-1 text-xs font-semibold text-slate-400">Alert type</div>
            {(Object.keys(SOURCE_LABEL) as AlertSource[]).map(src => (
              <label key={src} className="flex items-center justify-between py-1 text-xs text-slate-300">
                <span>{SOURCE_LABEL[src]}</span>
                <input type="checkbox" checked={settings.sources[src]} onChange={() => toggleSource(src)} className="h-4 w-4" />
              </label>
            ))}
          </div>

          <div className="mb-4">
            <div className="mb-1 text-xs font-semibold text-slate-400">Severity</div>
            {(["critical", "warning", "info"] as AlertSeverity[]).map(sev => (
              <label key={sev} className="flex items-center justify-between py-1 text-xs capitalize text-slate-300">
                <span>{sev}</span>
                <input type="checkbox" checked={settings.severities[sev]} onChange={() => toggleSeverity(sev)} className="h-4 w-4" />
              </label>
            ))}
          </div>

          {hostsSeen.length > 0 && (
            <div>
              <div className="mb-1 text-xs font-semibold text-slate-400">Mute specific hosts</div>
              <div className="max-h-32 space-y-1 overflow-y-auto">
                {hostsSeen.map(h => (
                  <label key={h} className="flex items-center justify-between py-1 text-xs text-slate-300">
                    <span className="truncate">{h}</span>
                    <input type="checkbox" checked={settings.mutedHostnames.includes(h)} onChange={() => toggleMuteHost(h)} className="h-4 w-4" />
                  </label>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      <button
        onClick={() => setPanelOpen(o => !o)}
        className={`flex h-12 w-12 items-center justify-center rounded-full border shadow-lg ${
          downCount > 0 ? "border-red-800 bg-red-950 text-red-300" : "border-slate-700 bg-slate-900 text-slate-400"
        }`}
        title="Voice alert settings"
      >
        <span className="text-lg">{settings.enabled ? "🔔" : "🔕"}</span>
        {downCount > 0 && <span className="absolute -mt-6 -mr-6 rounded-full bg-red-600 px-1.5 text-[10px] font-bold text-white">{downCount}</span>}
      </button>
    </div>
  );
}
