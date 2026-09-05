// Shared form primitives for the GitHub-dark design system: text inputs,
// textareas, selects and a label wrapper, plus a couple of layout helpers
// (PageHeader, Banner) used by every admin-style page.

import { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";

const fieldBase =
  "mt-1 w-full rounded-[6px] border border-[#30363D] bg-[#0D1117] px-3 py-2 text-sm text-[#E6EDF3] outline-none transition placeholder:text-[#484F58] focus:border-[#58A6FF]";

export function Input(props: InputHTMLAttributes<HTMLInputElement> & { className?: string }) {
  const { className = "", ...rest } = props;
  return <input className={`${fieldBase} ${className}`} {...rest} />;
}

export function Textarea(props: TextareaHTMLAttributes<HTMLTextAreaElement> & { className?: string }) {
  const { className = "", ...rest } = props;
  return <textarea className={`${fieldBase} ${className}`} {...rest} />;
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement> & { className?: string }) {
  const { className = "", ...rest } = props;
  return <select className={`${fieldBase} ${className}`} {...rest} />;
}

export function FieldLabel({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return <label className={`text-sm text-[#8B949E] ${className}`}>{children}</label>;
}

export function Checkbox(props: InputHTMLAttributes<HTMLInputElement>) {
  const { className = "", ...rest } = props;
  return <input type="checkbox" className={`h-4 w-4 accent-[#238636] ${className}`} {...rest} />;
}

/** Standard page banner: overline label + h1 title + description. */
export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow: string;
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <div className="label text-[#8B949E]">{eyebrow}</div>
        <h1 className="mt-1 text-[22px] font-bold tracking-[-0.5px] text-[#E6EDF3]">{title}</h1>
        {description && <p className="mt-1 max-w-3xl text-xs text-[#8B949E]">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
    </div>
  );
}

/** Inline status/error message banner. */
export function Banner({ children, tone = "info" }: { children: React.ReactNode; tone?: "info" | "error" }) {
  const cls =
    tone === "error"
      ? "border-[#672525] bg-[#2D1212] text-[#F78166]"
      : "border-[#1B4B91] bg-[#11233F] text-[#79C0FF]";
  return <div className={`mb-5 rounded-[6px] border px-4 py-3 text-sm ${cls}`}>{children}</div>;
}

/** A dark inner panel used to group related fields (e.g. within a form). */
export function Panel({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`rounded-[6px] border border-[#21262D] bg-[#0D1117] p-4 ${className}`}>
      {children}
    </div>
  );
}
