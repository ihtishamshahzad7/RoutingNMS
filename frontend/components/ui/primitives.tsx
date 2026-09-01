// Small primitives: AlertBadge, Tag, AiBadge, Button.

export function AlertBadge({ count }: { count: number }) {
  return (
    <span className="inline-flex items-center rounded-[20px] bg-[#F78166] px-[7px] py-[2px] text-[10px] font-bold text-[#0D1117]">
      {count}
    </span>
  );
}

export function Tag({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center rounded-[3px] bg-[#21262D] px-[5px] py-[1px] text-[9px] text-[#8B949E]">
      {children}
    </span>
  );
}

export function AiBadge() {
  return (
    <span className="inline-flex items-center rounded-[20px] border border-[#6E40C9] bg-[#1A1140] px-[7px] py-[2px] text-[9px] font-bold uppercase text-[#A371F7]">
      AI
    </span>
  );
}

type ButtonVariant = "primary" | "secondary" | "danger";

export function Button({
  variant = "secondary",
  className = "",
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }) {
  const variants: Record<ButtonVariant, string> = {
    primary:
      "bg-[#238636] border-[#2EA043] text-white hover:bg-[#2EA043]",
    secondary:
      "bg-[#21262D] border-[#30363D] text-[#E6EDF3] hover:bg-[#1C2128]",
    danger:
      "bg-[#2D1212] border-[#672525] text-[#F78166] hover:bg-[#3A1818]",
  };
  return (
    <button
      className={`inline-flex items-center gap-1.5 rounded-[5px] border px-3 py-1.5 text-[11px] font-semibold transition-colors duration-100 disabled:opacity-50 ${variants[variant]} ${className}`}
      {...props}
    >
      {children}
    </button>
  );
}
