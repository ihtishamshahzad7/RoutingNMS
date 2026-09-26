// Standalone status dot (7px) with optional pulse, used in feed rows,
// top bar and device cards.
//
// Feature 0.5 (Design System): color mapping now comes from the shared
// statusColors.ts table instead of a copy of it kept only in this file --
// see that file's header comment for why. No visual or behavioral change.

import { normalizeStatusKey, PULSE_STATUSES, STATUS_COLOR } from "./statusColors";

export function StatusDot({
  status,
  pulse = false,
}: {
  status: string;
  pulse?: boolean;
}) {
  const key = normalizeStatusKey(status);
  const color = STATUS_COLOR[key] ?? STATUS_COLOR.unknown;
  const pulseCls = pulse && PULSE_STATUSES.has(key);
  return (
    <span
      className="inline-block h-[7px] w-[7px] shrink-0 rounded-full"
      style={{
        background: color,
        boxShadow: `0 0 6px ${color}66`,
        animation: pulseCls ? "dot-pulse 2s ease-in-out infinite" : undefined,
      }}
    />
  );
}
