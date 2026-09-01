// Standalone status dot (7px) with optional pulse, used in feed rows,
// top bar and device cards.

export function StatusDot({
  status,
  pulse = false,
}: {
  status: string;
  pulse?: boolean;
}) {
  const key = (status || "").toLowerCase().trim();
  const map: Record<string, string> = {
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
  const color = map[key] ?? "#8B949E";
  const pulseCls = pulse && (key === "critical" || key === "down" || key === "warning" || key === "open" || key === "degraded");
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
