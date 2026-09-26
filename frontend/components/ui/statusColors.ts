// Feature 0.5 (Design System): single source of truth for status -> color
// mapping, consumed by both StatusDot and StatusPill.
//
// Before this change, StatusDot and StatusPill each hard-coded their own
// copy of this status-key -> hex-color map (confirmed by reading both
// files side by side: identical key sets, identical hex values, just
// duplicated). That's a real drift risk for a design system feature to
// close: change a status color in one file and the other silently goes
// stale. This file is the one place status colors are now defined; both
// components import from it instead of keeping their own copy.
//
// The hex values themselves are unchanged from what was already shipping
// (this is a consolidation, not a redesign) and correspond to the status
// tokens in app/globals.css's @theme block (--color-status-up, etc.) --
// kept as a plain object here rather than reading CSS variables at
// runtime, since these values also drive inline SVG stroke/fill colors
// (StatusDot's box-shadow) that need a real color string, not a CSS var
// reference, for correct rendering.
export const STATUS_COLOR: Record<string, string> = {
  up: "#3FB950",
  healthy: "#3FB950",
  reachable: "#3FB950",
  running: "#3FB950",
  resolved: "#3FB950",
  enabled: "#3FB950",
  warning: "#D29922",
  warn: "#D29922",
  degraded: "#D29922",
  acknowledged: "#D29922",
  critical: "#F78166",
  down: "#F78166",
  open: "#F78166",
  unknown: "#8B949E",
  pending: "#8B949E",
  disabled: "#8B949E",
  info: "#58A6FF",
  analyzing: "#A371F7",
};

// Pill background is a darker tint of the same hue, not derivable from the
// dot color alone (it's a distinct design choice per status, e.g. info's
// pill background is a blue-black, not a dimmed version of #58A6FF) -- so
// this stays its own table rather than being computed from STATUS_COLOR.
export const STATUS_PILL_BG: Record<string, string> = {
  up: "#12261E",
  healthy: "#12261E",
  reachable: "#12261E",
  running: "#12261E",
  resolved: "#12261E",
  enabled: "#12261E",
  warning: "#2D2000",
  warn: "#2D2000",
  degraded: "#2D2000",
  acknowledged: "#2D2000",
  critical: "#2D1212",
  down: "#2D1212",
  open: "#2D1212",
  unknown: "#1C2128",
  pending: "#1C2128",
  disabled: "#1C2128",
  info: "#11233F",
  analyzing: "#1A1140",
};

export function normalizeStatusKey(status: string): string {
  return (status || "").toLowerCase().trim();
}

export const PULSE_STATUSES = new Set(["critical", "down", "warning", "open", "degraded"]);
