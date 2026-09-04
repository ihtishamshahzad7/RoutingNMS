// Client-side settings for the browser voice-alert feature. Stored in
// localStorage (this is a real deployed page, not an in-conversation
// preview, so localStorage is the right tool -- per-browser, survives
// reloads, no backend round trip needed for a purely per-operator
// preference like "how loud" or "how often to repeat").

export type AlertSource = "olt" | "device" | "trap" | "http" | "icmp";
export type AlertSeverity = "critical" | "warning" | "info";

export type VoiceAlertSettings = {
  enabled: boolean;
  volume: number; // 0..1
  repeatIntervalSeconds: number; // how often an still-active alert is re-spoken
  sources: Record<AlertSource, boolean>; // "alert type" filter -- which subsystem to announce
  severities: Record<AlertSeverity, boolean>;
  mutedHostnames: string[]; // per-device/OLT mute list
  announceRecoveries: boolean; // speak "back up" events, not just "down" ones -- independent of the severity filter, since a recovery is reported as info-level regardless of how severe the original outage was
};

export const DEFAULT_VOICE_ALERT_SETTINGS: VoiceAlertSettings = {
  enabled: true,
  volume: 1,
  repeatIntervalSeconds: 120,
  sources: { olt: true, device: true, trap: true, http: true, icmp: true },
  severities: { critical: true, warning: true, info: false },
  mutedHostnames: [],
  announceRecoveries: true,
};

const STORAGE_KEY = "routingnms.voiceAlertSettings";

export function loadVoiceAlertSettings(): VoiceAlertSettings {
  if (typeof window === "undefined") return DEFAULT_VOICE_ALERT_SETTINGS;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_VOICE_ALERT_SETTINGS;
    const parsed = JSON.parse(raw);
    return {
      ...DEFAULT_VOICE_ALERT_SETTINGS,
      ...parsed,
      sources: { ...DEFAULT_VOICE_ALERT_SETTINGS.sources, ...parsed.sources },
      severities: { ...DEFAULT_VOICE_ALERT_SETTINGS.severities, ...parsed.severities },
      mutedHostnames: Array.isArray(parsed.mutedHostnames) ? parsed.mutedHostnames : [],
    };
  } catch {
    return DEFAULT_VOICE_ALERT_SETTINGS;
  }
}

export function saveVoiceAlertSettings(settings: VoiceAlertSettings) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  } catch {
    // best-effort -- a private window or full storage quota shouldn't break the page
  }
}
