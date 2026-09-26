// Feature 0.5 (Design System): a shared, token-based modal/dialog
// primitive for the GitHub-dark design system.
//
// Before this feature, no reusable Modal existed in components/ui/ at
// all -- the one modal dialog in this codebase (the Add/Test/Edit SNMP
// dialogs on the Devices page) is a private, page-local `Modal` function
// hand-rolled with a completely different color palette (slate-800/900/
// 950 + cyan accents, not this design system's GitHub-dark tokens) from
// the external "redesign device inventory" commit noted in
// claude/roadmap.md. That page's modal is deliberately left untouched
// here (it works, and rewriting it risks disrupting a working flow for
// no user-visible benefit) -- this is the shared primitive future
// features should reach for instead of hand-rolling another one-off.
export function Modal({
  title,
  subtitle,
  onClose,
  children,
  maxWidth = "max-w-2xl",
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: React.ReactNode;
  maxWidth?: string;
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className={`max-h-[92vh] w-full ${maxWidth} overflow-y-auto rounded-[10px] border border-border-strong bg-bg-surface shadow-2xl`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="sticky top-0 z-10 flex items-start justify-between border-b border-border bg-bg-surface/95 px-6 py-5 backdrop-blur">
          <div>
            <h2 className="text-[15px] font-bold text-text-primary">{title}</h2>
            {subtitle && <p className="mt-1 text-xs text-text-muted">{subtitle}</p>}
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="rounded-[5px] p-1.5 text-text-muted transition-colors hover:bg-bg-raised hover:text-text-primary"
          >
            ✕
          </button>
        </div>
        <div className="p-6">{children}</div>
      </div>
    </div>
  );
}
