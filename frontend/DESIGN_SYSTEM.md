# RoutingNMS Design System

Feature 0.5 of the RoutingNMS build blueprint. This is a pointer, not a
duplicate of the real thing — the design system lives in code, and this
file just says where.

## Where it actually lives

- **Tokens**: `app/globals.css`'s `@theme` block — colors, fonts, radii.
  Every token generates a Tailwind utility automatically (`--color-bg-surface`
  → `bg-bg-surface`, `text-bg-surface`, `border-bg-surface`, etc.) — use the
  utility class, don't hardcode a hex value in a new component.
- **Components**: `components/ui/` — `primitives.tsx` (Button, Tag, AiBadge,
  AlertBadge), `card.tsx` (Card, StatCard), `form.tsx` (Input, Select,
  Textarea, FieldLabel, Checkbox, PageHeader, Banner, Panel), `status-dot.tsx`
  / `status-pill.tsx` (status indicators, both backed by the shared
  `statusColors.ts` table), `sparkline.tsx`, `modal.tsx`, `tooltip.tsx`.
- **Living reference**: `/style-guide` (in the app, under the Account nav
  group) renders every token swatch and every component above with real
  examples. It's the actual source of truth for what "looks right" —
  breaks visibly in the browser if a token or component regresses, the
  same real-hardware-testable standard the rest of this build blueprint
  holds backend features to.

## Before adding something new

Check `/style-guide` and `components/ui/` first. If what you need already
exists, use it. If it's genuinely missing, add it to `components/ui/` (not
inline in your page) so the next feature finds it too — that's the whole
point of a shared library. Two real duplication bugs this feature found
and fixed: `status-dot.tsx` and `status-pill.tsx` each carried their own
copy of the status→color map (now one shared table in `statusColors.ts`);
and the Devices page's SNMP dialogs use a private, differently-themed
`Modal` (slate/cyan, not this system's GitHub-dark tokens) predating this
design system — left as-is since it works and rewriting it risks breaking
a live flow, but `components/ui/modal.tsx` is now here so nothing else
repeats that pattern.

## Known gaps (flagged, not addressed here — 0.5b)

- No lint rule catches a new component hardcoding a hex color instead of a
  token utility class — this pass fixed the two duplicates it found by
  reading the code, not by tooling that prevents new ones.
- No `<table>` wrapper component exists yet, despite 14 pages hand-rolling
  their own `<table>` markup — a shared `Table`/`DataTable` primitive is a
  reasonable next addition if a future feature needs one.
- The Devices page's SNMP onboarding modal (slate/cyan palette) was not
  migrated onto `components/ui/modal.tsx` or the GitHub-dark tokens, per
  the "don't disrupt a working flow" call above.
