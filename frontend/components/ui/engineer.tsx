"use client";

// "Engineer Classic" design primitives — light, navy-sidebar, dense flat
// cards (SolarWinds/PRTG-inspired). Design direction locked by the user
// for the 6-page rebuild; tokens below are the literal values given,
// used with no substitutions. This file is additive (the existing dark
// "GitHub-dark" design system in components/ui/* is untouched) so pages
// outside the 6-page scope keep rendering exactly as before.

import { InputHTMLAttributes, ReactNode, useState } from "react";

export const ENG = {
  pageBg: "#F4F6F9",
  text: "#1F2A37",
  sidebarBg: "#1B2A41",
  sidebarText: "#C4CDD9",
  sidebarActiveBg: "#26374F",
  sidebarActiveBorder: "#2E7BF6",
  topbarBg: "#FFFFFF",
  topbarBorder: "#DCE1E8",
  topbarText: "#5C6B7A",
  panelBg: "#FFFFFF",
  panelBorder: "#DCE1E8",
  panelHeaderBg: "#F9FAFC",
  rowDivider: "#EEF1F4",
  up: "#1E8E5A",
  warn: "#C77700",
  down: "#C4362D",
  accent: "#2E7BF6",
} as const;

/** Flat white card/panel: 1px border, 4px radius, optional labeled header. */
export function EngPanel({
  title,
  actions,
  children,
  className = "",
}: {
  title?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-[4px] border border-[#DCE1E8] bg-white ${className}`}>
      {title && (
        <div className="flex items-center justify-between rounded-t-[4px] border-b border-[#DCE1E8] bg-[#F9FAFC] px-4 py-2.5">
          <h2 className="text-[13px] font-medium text-[#1F2A37]">{title}</h2>
          {actions}
        </div>
      )}
      <div className="p-3.5">{children}</div>
    </div>
  );
}

/** Compact top-aligned-label field wrapper -- label sits directly above the
 * control, not in a separate column, per the "Forms" spec. */
export function EngField({
  label,
  htmlFor,
  error,
  hint,
  children,
  wide,
}: {
  label: string;
  htmlFor?: string;
  error?: string;
  hint?: string;
  children: ReactNode;
  wide?: boolean;
}) {
  return (
    <div className={wide ? "sm:col-span-2" : ""}>
      <label htmlFor={htmlFor} className="mb-1 block text-[11px] font-medium text-[#5C6B7A]">
        {label}
      </label>
      {children}
      {error ? (
        <p className="mt-1 text-[11px] text-[#C4362D] transition-opacity duration-150">{error}</p>
      ) : hint ? (
        <p className="mt-1 text-[11px] text-[#8A96A3]">{hint}</p>
      ) : null}
    </div>
  );
}

const engInputBase =
  "h-[33px] w-full rounded-[4px] border bg-white px-2.5 text-[13px] font-normal text-[#1F2A37] outline-none transition-colors duration-150 placeholder:text-[#9AA6B2] focus:border-[#2E7BF6]";

export function EngInput({
  className = "",
  error,
  ...rest
}: InputHTMLAttributes<HTMLInputElement> & { error?: boolean; className?: string }) {
  return (
    <input
      className={`${engInputBase} ${error ? "border-[#C4362D]" : "border-[#DCE1E8]"} ${className}`}
      {...rest}
    />
  );
}

/** A plain <select>, same box as EngInput. */
export function EngSelect({
  className = "",
  children,
  ...rest
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select className={`${engInputBase} border-[#DCE1E8] ${className}`} {...rest}>
      {children}
    </select>
  );
}

/** Toggle switch, replaces plain checkboxes for boolean options. Renders a
 * real (visually hidden) checkbox so it still posts through a native
 * <form>'s FormData under `name`, same as the checkbox it replaces. */
export function EngToggle({
  name,
  defaultChecked,
  checked,
  onChange,
  label,
  description,
}: {
  name: string;
  defaultChecked?: boolean;
  checked?: boolean;
  onChange?: (checked: boolean) => void;
  label: string;
  description?: string;
}) {
  const [internal, setInternal] = useState(!!defaultChecked);
  const isOn = checked ?? internal;
  return (
    <label className="flex cursor-pointer items-center justify-between gap-4 py-1">
      <span>
        <span className="block text-[13px] font-medium text-[#1F2A37]">{label}</span>
        {description && <span className="mt-0.5 block text-[11px] text-[#5C6B7A]">{description}</span>}
      </span>
      <span className="relative inline-flex shrink-0 items-center">
        <input
          type="checkbox"
          name={name}
          checked={isOn}
          onChange={(e) => {
            setInternal(e.target.checked);
            onChange?.(e.target.checked);
          }}
          className="peer sr-only"
        />
        <span
          className={`h-[18px] w-[32px] rounded-full transition-colors duration-150 ${
            isOn ? "bg-[#2E7BF6]" : "bg-[#DCE1E8]"
          }`}
        />
        <span
          className={`absolute left-[2px] top-[2px] h-[14px] w-[14px] rounded-full bg-white shadow transition-transform duration-150 ${
            isOn ? "translate-x-[14px]" : "translate-x-0"
          }`}
        />
      </span>
    </label>
  );
}

/** Chip/tag input for multi-value fields (tags, accepted status codes, …) --
 * replaces a raw comma-separated text box. Posts as a hidden input carrying
 * a comma-joined string under `name`, so existing form-submit code that
 * reads FormData(form).get(name) keeps working unchanged. */
export function EngChipInput({
  name,
  defaultValue = [],
  placeholder,
}: {
  name: string;
  defaultValue?: string[];
  placeholder?: string;
}) {
  const [chips, setChips] = useState<string[]>(defaultValue);
  const [draft, setDraft] = useState("");

  function commit() {
    const v = draft.trim();
    if (v && !chips.includes(v)) setChips((c) => [...c, v]);
    setDraft("");
  }

  return (
    <div className="flex min-h-[33px] w-full flex-wrap items-center gap-1.5 rounded-[4px] border border-[#DCE1E8] bg-white px-2 py-1 focus-within:border-[#2E7BF6]">
      <input type="hidden" name={name} value={chips.join(",")} readOnly />
      {chips.map((c) => (
        <span
          key={c}
          className="inline-flex items-center gap-1 rounded-[3px] bg-[#EEF3FD] px-1.5 py-0.5 text-[11px] font-medium text-[#2E7BF6]"
        >
          {c}
          <button
            type="button"
            aria-label={`Remove ${c}`}
            onClick={() => setChips((cs) => cs.filter((x) => x !== c))}
            className="text-[#2E7BF6]/70 hover:text-[#2E7BF6]"
          >
            ×
          </button>
        </span>
      ))}
      <input
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            commit();
          } else if (e.key === "Backspace" && !draft && chips.length) {
            setChips((c) => c.slice(0, -1));
          }
        }}
        onBlur={commit}
        placeholder={chips.length ? "" : placeholder}
        className="min-w-[80px] flex-1 border-none bg-transparent text-[13px] text-[#1F2A37] outline-none placeholder:text-[#9AA6B2]"
      />
    </div>
  );
}

/** IPv4 validator used for real-time inline validation on address fields. */
export function isValidIPv4(v: string): boolean {
  if (!v) return true; // empty is "not yet invalid" -- required-ness is separate
  const m = v.trim().match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (!m) return false;
  return m.slice(1).every((p) => Number(p) >= 0 && Number(p) <= 255);
}

/** Modal with a sticky footer -- Save/Cancel stay visible on a long,
 * scrolling form instead of scrolling out of view. */
export function EngModal({
  title,
  subtitle,
  onClose,
  children,
  footer,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
  footer: ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[#1F2A37]/50 p-4" onClick={onClose}>
      <div
        className="flex max-h-[88vh] w-full max-w-lg flex-col overflow-hidden rounded-[4px] border border-[#DCE1E8] bg-white shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex shrink-0 items-start justify-between border-b border-[#DCE1E8] bg-[#F9FAFC] px-5 py-3">
          <div>
            <h2 className="text-[13px] font-medium text-[#1F2A37]">{title}</h2>
            {subtitle && <p className="mt-0.5 text-[11px] text-[#5C6B7A]">{subtitle}</p>}
          </div>
          <button
            onClick={onClose}
            aria-label="Close"
            className="rounded-[3px] p-1 text-[#5C6B7A] transition-colors duration-150 hover:bg-[#EEF1F4] hover:text-[#1F2A37]"
          >
            ✕
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">{children}</div>
        <div className="flex shrink-0 items-center justify-end gap-2 border-t border-[#DCE1E8] bg-white px-5 py-3">
          {footer}
        </div>
      </div>
    </div>
  );
}

export function EngButton({
  variant = "secondary",
  className = "",
  ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary" }) {
  const base = "inline-flex h-[33px] items-center gap-1.5 rounded-[4px] px-3.5 text-[13px] font-medium transition-colors duration-150 disabled:opacity-50";
  const cls =
    variant === "primary"
      ? `${base} bg-[#2E7BF6] text-white hover:bg-[#2568D4]`
      : `${base} border border-[#DCE1E8] bg-white text-[#1F2A37] hover:bg-[#F4F6F9]`;
  return <button className={`${cls} ${className}`} {...rest} />;
}

/** A labeled section that groups related fields (e.g. "Connection",
 * "Monitoring") instead of one long flat list. */
export function EngSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset className="rounded-[4px] border border-[#DCE1E8] p-3.5">
      <legend className="px-1 text-[11px] font-medium uppercase tracking-wide text-[#5C6B7A]">{title}</legend>
      <div className="grid gap-3 sm:grid-cols-2">{children}</div>
    </fieldset>
  );
}
