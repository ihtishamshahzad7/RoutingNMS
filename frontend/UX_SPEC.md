# RoutingNMS Core NOC Screens — UI/UX Spec

Feature 0.6 of the RoutingNMS build blueprint, the last of Phase 0. Read
`DESIGN_SYSTEM.md` first (tokens/components/`/style-guide`) — this doc is
the layer above that: how a *screen*, not a component, should be put
together, and an audit of the real core screens against it.

## Screen anatomy contract

Every page under `app/(noc)/` renders inside the shared `NocLayout`
(`app/(noc)/layout.tsx`: persistent `Sidebar` + 40px `TopBar`, its own
`overflow-y-auto` scroll container). Because of that:

- **Never** wrap a page's own content in `min-h-screen` or another
  full-viewport-height class — the layout's scroll container already
  handles height/scroll; a nested `min-h-screen` fights it.
- **Never** hand-roll a second top bar or page-level header chrome —
  `TopBar` already provides that. A page's own header is just its title
  block (`PageHeader` from `components/ui/form.tsx`, or the equivalent
  eyebrow/h1/description markup topology/page.tsx and others use).
- Standard outer wrapper: `<main className="mx-auto max-w-7xl px-6 py-6">`
  (or `p-6 md:p-8` — both conventions are in real use; either is fine, just
  don't mix `min-h-screen` in).
- Use real tokens (`bg-bg-surface`, `text-text-primary`, `border-border`,
  etc., or the equivalent `.tbl`/`.input`/`.label` utility classes already
  in `app/globals.css`) — never a shadcn-convention class
  (`text-muted-foreground`, `bg-background`, `bg-primary`,
  `text-primary-foreground`) that this theme doesn't define. An undefined
  utility class doesn't error, it just silently renders unstyled — this is
  exactly the bug this feature found and fixed on the Incidents screen
  (see below).

## Screen states every core screen should have

- **Loading**: a visible loading state before first data arrives (a
  message row, not a blank table — every core screen audited below
  already does this).
- **Empty**: a distinct "no results" message, not an empty table with no
  explanation.
- **Error**: a visible, non-blocking error state — most core screens
  either keep showing the last-known-good data on a failed refresh
  (`catch { /* keep last list */ }`, e.g. Incidents, Reachability) or show
  a dedicated error panel (e.g. the OLT detail screen). Both are
  acceptable; a silent failure with no on-screen indication is not.

## Audit of core NOC screens against this contract

| Screen | File | Status |
|---|---|---|
| Dashboard | `dashboard/page.tsx` | Compliant — real tokens, `PageHeader`-equivalent header, loading/empty states per widget. |
| Devices | `devices/page.tsx` | Compliant on the list view. |
| Reachability | `reachability/page.tsx` | Compliant — real tokens, loading/empty states present. |
| Topology | `topology/page.tsx` | Compliant — the reference pattern this doc's "standard outer wrapper" is drawn from. |
| Alert Rules | `alert-rules/page.tsx` | Compliant. |
| **Incidents** | `incidents/page.tsx` | **Fixed in this feature.** Previously used undefined shadcn-style classes (`text-muted-foreground`, `bg-background`, `bg-primary`, `text-primary-foreground` — none defined in `app/globals.css`'s `@theme`, so they silently rendered unstyled) and a redundant/conflicting `min-h-screen` wrapper inside the layout's own scroll container. Rewritten onto `PageHeader`, `StatCard`, `StatusPill`, `Button`, and the `.tbl`/`.input` utility classes — same data-fetching logic, same endpoints, same actions, purely a structural/style fix. |

## Known gaps (flagged, not addressed here — 0.6b)

AGENTS.md's own Sprint 4 checklist already named this exact gap before
this feature started ("Light token pass still pending on: sites,
access-points, customers, syslog, traps, mibs, provisioning,
devices/[id], incidents, olts/[id]") — confirmed still true by reading
each file. This feature closed the one item on that list that was
actually broken (Incidents' undefined classes), plus documented the
contract above so the rest get fixed consistently. The remaining nine are
NOT broken — they render correctly on a different, internally-consistent
palette (mostly slate/cyan/emerald, e.g. `olts/[id]/page.tsx`) — just
cosmetically inconsistent with the rest of the app, and each is
200+ lines, large enough that retrofitting all nine in one pass risked
more than this increment's budget justified. Left as explicit follow-up,
in priority order for a future increment: `devices/[id]/page.tsx` and
`olts/[id]/page.tsx` (both core monitoring detail screens, most
user-facing) before the more peripheral admin pages (`sites`,
`access-points`, `customers`, `syslog`, `traps`, `mibs`, `provisioning`).

Also not addressed: no automated check (lint rule or otherwise) catches a
*new* page introducing an undefined utility class the way Incidents did —
the `/style-guide` page (feature 0.5) helps a developer eyeball the real
palette, but doesn't scan other pages for drift.
