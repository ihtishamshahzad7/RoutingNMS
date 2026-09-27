"use client";

// Kuma-style heartbeat bar: renders the last ~50 probe results as small
// colored bars (green = up, red = down, amber = degraded/loss), most
// recent on the right, matching Uptime Kuma's monitor-list heartbeat.
//
// Item 1 of the Kuma core-parity freeze (2026-09-27). Deliberately a small,
// dependency-free component -- no new package.

export type Beat = { reachable: boolean; lossPct?: number; probedAt?: string };

const MAX_BEATS = 50;

function beatColor(b: Beat): string {
  if (!b.reachable) return "#C4362D"; // down
  if ((b.lossPct ?? 0) > 0) return "#C77700"; // degraded/loss, still reachable
  return "#1E8E5A"; // up
}

export function HeartbeatBar({ beats, height = 28 }: { beats: Beat[]; height?: number }) {
  // History arrives oldest-first from the API; Kuma draws oldest-on-the-left,
  // most-recent-on-the-right. Pad on the left with empty placeholders so a
  // monitor with little history doesn't look artificially "denser" than one
  // with a full 50.
  const trimmed = beats.slice(-MAX_BEATS);
  const padding = Math.max(0, MAX_BEATS - trimmed.length);

  return (
    <div className="flex items-end gap-[2px]" style={{ height }} title={`${trimmed.length} of last ${MAX_BEATS} checks`}>
      {Array.from({ length: padding }).map((_, i) => (
        <span key={`pad-${i}`} className="w-[3px] shrink-0 rounded-[1px] bg-[#DCE1E8]" style={{ height: "100%" }} />
      ))}
      {trimmed.map((b, i) => (
        <span
          key={i}
          className="w-[3px] shrink-0 rounded-[1px]"
          style={{ height: "100%", background: beatColor(b) }}
          title={b.probedAt ? new Date(b.probedAt).toLocaleString() : undefined}
        />
      ))}
    </div>
  );
}
