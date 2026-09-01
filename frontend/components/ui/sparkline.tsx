"use client";

// Dependency-free SVG sparkline (reinforces the project's hand-rolled SVG
// convention for tiny charts). Used for per-device RTT history in the
// Top Issues column and device cards. No animation - always live data.

export function Sparkline({
  points,
  width = 160,
  height = 24,
  stroke = "#58A6FF",
}: {
  points: number[];
  width?: number;
  height?: number;
  stroke?: string;
}) {
  if (!points || points.length === 0) {
    return (
      <svg width={width} height={height} className="opacity-40" aria-hidden="true" />
    );
  }
  const pad = 2;
  const values = points;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const stepX = values.length > 1 ? (width - pad * 2) / (values.length - 1) : 0;
  const coords = values.map((v, i) => {
    const x = pad + i * stepX;
    const y = height - pad - ((v - min) / range) * (height - pad * 2);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  return (
    <svg width={width} height={height} aria-hidden="true">
      <polyline
        points={coords.join(" ")}
        fill="none"
        stroke={stroke}
        strokeWidth={1.5}
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
