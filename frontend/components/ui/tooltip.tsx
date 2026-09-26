// Feature 0.5 (Design System): a shared hover-tooltip primitive.
//
// Before this feature, this codebase's only tooltip was a one-off native
// SVG <title> element in the Workspace Topology Builder's link-validation
// canvas (feature 37) -- fine for an SVG line, but no reusable HTML
// tooltip existed for ordinary page content (icon buttons, truncated
// labels, etc.). This is a small, dependency-free hover tooltip (no
// portal, no positioning library) suitable for that common case; it is
// NOT a replacement for the SVG <title> approach feature 37 already uses
// on canvas elements, which stays as-is.
import { useState } from "react";

export function Tooltip({
  label,
  children,
  side = "top",
}: {
  label: string;
  children: React.ReactNode;
  side?: "top" | "bottom";
}) {
  const [open, setOpen] = useState(false);
  const sideCls = side === "top" ? "bottom-full mb-1.5" : "top-full mt-1.5";
  return (
    <span
      className="relative inline-flex"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocus={() => setOpen(true)}
      onBlur={() => setOpen(false)}
    >
      {children}
      {open && (
        <span
          role="tooltip"
          className={`pointer-events-none absolute left-1/2 z-50 -translate-x-1/2 whitespace-nowrap rounded-[5px] border border-border-strong bg-bg-raised px-2 py-1 text-[10px] font-medium text-text-primary shadow-lg ${sideCls}`}
        >
          {label}
        </span>
      )}
    </span>
  );
}
