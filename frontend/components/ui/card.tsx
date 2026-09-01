// Card + StatCard primitives following the design system.

export function Card({
  children,
  className = "",
  title,
  headerRight,
}: {
  children: React.ReactNode;
  className?: string;
  title?: React.ReactNode;
  headerRight?: React.ReactNode;
}) {
  return (
    <section
      className={`rounded-[8px] border border-[#21262D] bg-[#161B22] ${className}`}
    >
      {title && (
        <div className="flex items-center justify-between border-b border-[#21262D] px-4 py-3">
          <div className="text-sm font-semibold text-[#E6EDF3]">{title}</div>
          {headerRight}
        </div>
      )}
      {children}
    </section>
  );
}

export function StatCard({
  label,
  value,
  sub,
  accent = "text-[#E6EDF3]",
  unit,
}: {
  label: string;
  value: React.ReactNode;
  sub?: React.ReactNode;
  accent?: string;
  unit?: string;
}) {
  return (
    <div className="rounded-[8px] border border-[#21262D] bg-[#161B22] px-4 py-3">
      <div className="label">{label}</div>
      <div className={`mt-1 font-mono text-2xl font-bold leading-none ${accent}`}>
        {value}
        {unit && <span className="ml-1 text-sm font-normal text-[#8B949E]">{unit}</span>}
      </div>
      {sub && <div className="mt-1 text-[10px] text-[#484F58]">{sub}</div>}
    </div>
  );
}
