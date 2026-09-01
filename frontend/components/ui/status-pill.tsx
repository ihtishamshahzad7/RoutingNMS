// Reusable NOC primitive: a small status pill ("● Up", "● Critical", ...).
// Pure presentational; callers pass the status key and it maps to the
// design-system colors.

const STATUS_CLASSES: Record<string, string> = {
  up: "bg-[#12261E] text-[#3FB950]",
  healthy: "bg-[#12261E] text-[#3FB950]",
  reachable: "bg-[#12261E] text-[#3FB950]",
  running: "bg-[#12261E] text-[#3FB950]",
  warning: "bg-[#2D2000] text-[#D29922]",
  warn: "bg-[#2D2000] text-[#D29922]",
  degraded: "bg-[#2D2000] text-[#D29922]",
  critical: "bg-[#2D1212] text-[#F78166]",
  down: "bg-[#2D1212] text-[#F78166]",
  unknown: "bg-[#1C2128] text-[#8B949E]",
  pending: "bg-[#1C2128] text-[#8B949E]",
  open: "bg-[#2D1212] text-[#F78166]",
  resolved: "bg-[#12261E] text-[#3FB950]",
  acknowledged: "bg-[#2D2000] text-[#D29922]",
  enabled: "bg-[#12261E] text-[#3FB950]",
  disabled: "bg-[#1C2128] text-[#8B949E]",
  info: "bg-[#11233F] text-[#58A6FF]",
  analyzing: "bg-[#1A1140] text-[#A371F7]",
};

const DOT_CLASSES: Record<string, string> = {
  up: "bg-[#3FB950]",
  healthy: "bg-[#3FB950]",
  reachable: "bg-[#3FB950]",
  running: "bg-[#3FB950]",
  warning: "bg-[#D29922]",
  warn: "bg-[#D29922]",
  degraded: "bg-[#D29922]",
  critical: "bg-[#F78166]",
  down: "bg-[#F78166]",
  unknown: "bg-[#8B949E]",
  pending: "bg-[#8B949E]",
  open: "bg-[#F78166]",
  resolved: "bg-[#3FB950]",
  acknowledged: "bg-[#D29922]",
  enabled: "bg-[#3FB950]",
  disabled: "bg-[#8B949E]",
  info: "bg-[#58A6FF]",
  analyzing: "bg-[#A371F7]",
};

export function normalizeStatus(status: string): string {
  return (status || "").toLowerCase().trim();
}

export function StatusPill({
  status,
  label,
  pulse = false,
}: {
  status: string;
  label?: string;
  pulse?: boolean;
}) {
  const key = normalizeStatus(status);
  const cls = STATUS_CLASSES[key] ?? STATUS_CLASSES.unknown;
  const dotCls = DOT_CLASSES[key] ?? DOT_CLASSES.unknown;
  const text = label ?? status;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[9px] font-bold uppercase tracking-[0.06em] ${cls}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${dotCls} ${pulse ? "animate-pulse" : ""}`} />
      {text}
    </span>
  );
}
