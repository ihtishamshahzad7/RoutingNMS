// Reusable NOC primitive: a small status pill ("● Up", "● Critical", ...).
// Pure presentational; callers pass the status key and it maps to the
// design-system colors.
//
// Feature 0.5 (Design System): color mapping now comes from the shared
// statusColors.ts table instead of a copy of it kept only in this file
// (previously a near-identical `STATUS_CLASSES`/`DOT_CLASSES` pair of
// Tailwind-arbitrary-value maps lived here, duplicating status-dot.tsx's
// own copy). Background/text/dot colors are applied via inline style
// rather than `bg-[#hex]` Tailwind arbitrary-value classes, since a
// dynamically-composed arbitrary-value class name isn't visible to
// Tailwind's static source scan and wouldn't generate -- inline style
// matches how StatusDot already renders its color for the same reason.
// No visual or behavioral change from the previous version.

import { normalizeStatusKey, PULSE_STATUSES, STATUS_COLOR, STATUS_PILL_BG } from "./statusColors";

export const normalizeStatus = normalizeStatusKey;

export function StatusPill({
  status,
  label,
  pulse = false,
}: {
  status: string;
  label?: string;
  pulse?: boolean;
}) {
  const key = normalizeStatusKey(status);
  const fg = STATUS_COLOR[key] ?? STATUS_COLOR.unknown;
  const bg = STATUS_PILL_BG[key] ?? STATUS_PILL_BG.unknown;
  const text = label ?? status;
  const shouldPulse = pulse && PULSE_STATUSES.has(key);
  return (
    <span
      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.06em]"
      style={{ background: bg, color: fg }}
    >
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{ background: fg, animation: shouldPulse ? "dot-pulse 2s ease-in-out infinite" : undefined }}
      />
      {text}
    </span>
  );
}
