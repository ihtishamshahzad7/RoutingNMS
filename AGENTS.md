# RoutingNMS — Durable Project Guidance

## What this project is

RoutingNMS is an AI-powered NMS for ISPs/MSPs/enterprises. Current stack:
- **Frontend**: Next.js (App Router) + TypeScript + Tailwind CSS v4 — `frontend/`
- **Backend**: Go (Go 1.24, module `github.com/ihtishamshahzad7/RoutingNMS/backend`) — `backend/`
- **Database**: PostgreSQL (migrations in `backend/migrations/*.sql`, idempotent)
- **Whole-program infra**: Redis, NATS, VictoriaMetrics (Docker Compose in `deployments/`)

This workspace is NOT a Python/FastAPI/SQLAlchemy codebase. Any build/feature work
must extend the **Go backend** and **Next.js frontend** in their existing idioms —
do not introduce Python, and do not rewrite what already works.

## Active build program: "RoutingNMS NextGen"

The user provided a large master build prompt (derived from other repos) and chose to
**map its features onto the existing Go/Next.js system** (not rewrite), building the
program **in order across sessions**. The prompt assumes a Python codebase; the Go
code already implements many of the same themes. Work the sprints below in order,
extending the Go backend + Next.js frontend, verifying with `go build` (Docker) and
`next build`.

### Sprint 0 — Foundation
- [x] Frontend is already Next.js (nothing to migrate)
- [x] Docker Compose already includes Redis + VictoriaMetrics (see `deployments/docker-compose.yml`)
- [x] Extend database schema — greenfield tables added as `backend/migrations/0013-0019*.sql`
      (ping_results, topology_links/snapshots, ai_incidents, alert_rules,
      notification_channels, sites/access_points/customer_connections, tenants/audit_logs;
      device icmp_*/syslog_enabled columns). VERIFY against live Postgres — SQL written to
      match idempotent existing pattern but not yet executed here.
- [x] Go ICMP ping poller — `backend/internal/ping` (poller.go, parse.go, api.go) +
      wiring + pruner in `backend/cmd/api/main.go` + `GET/POST /api/v1/ping/{id}/(live|history|probe)`.
      Frontend Ping section added to `frontend/app/(noc)/devices/[id]/page.tsx` (verified via `next build`).
      ⚠ not yet `go build` verified (see Verification).
- [ ] Expo mobile project init (later sprints depend on API, not on mobile)

### Next up (Sprint 4)
Multi-tenancy + audit log + hardening — see Sprint 4 checklist below. Sprint 3 (sites /
access points / customer connections CRUD + pages + provisioning extension) is done;
only mobile push notifications from Sprint 3 remain and they wait on the mobile client.

### Sprint 1 — Topology + Syslog (mostly exist; finish the gaps)
- [x] Wire periodic LLDP discovery loop; persist `topology_links` + snapshots
      (`backend/internal/topology`: engine.go = scheduled Discovery loop, repository.go =
      persist/upsert/stale/snapshot, inventory.go Graph() = persisted graph; wired in
      `backend/cmd/api/main.go` as `go topologyEngine.Run(ctx)` + routes
      `/api/topology` (persisted GraphHandler), `POST /api/v1/topology/discover`,
      `GET /api/v1/topology/status`, `GET /api/v1/topology/snapshots`; frontend
      `topology/page.tsx` adds discovery status + "Rediscover now"). ⚠ not yet `go build`
      verified (static review passed; no toolchain/Docker on this box, see Verification)
- [x] Syslog exist already (UDP+TCP receiver, parser, viewer) — no change needed
- [ ] Mobile device list + detail screens

### Sprint 2 — AI + Alert rules
- [x] Wire the dormant `backend/internal/alerts` engine: persist rules + API + UI
      (`alert_rules` from migration 0016; `alerts/repository.go` rules/channels CRUD,
      `alerts/evaluator.go` background loop over recent `metric_samples` honoring
      `for_duration_sec` + downsampled breach tracking, dedup via in-memory `Engine`;
      `alerts/api.go` multiplexer under `/api/v1/alerts/` for rules/channels/evaluator
      status; wired in `main.go` as `go alertEvaluator.Run(ctx)` with
      `ALERT_EVAL_INTERVAL_SECONDS` default 60; frontend `alert-rules/page.tsx` +
      sidebar NAV). ⚠ static review passed; not yet `go build` verified.
- [x] AI root-cause analysis on incidents: `ai_incidents` (migration 0015) is the durable
      store; `alerts/ai_incidents.go` `IncidentBridge` persists each fired incident +
      runs `rca.go` deterministic analyzer into `ai_incidents.rca_*`, then publishes to
      the live in-memory incident store + SSE stream; `alerts/rca.go` (root cause /
      confidence / affected services / recommended actions / impact / timeline / report).
      ⚠ heuristic RCA only — no external AI available offline.
- [x] Notification channels / multi-channel fanout (`notification_channels` from 0017;
      `alerts/notify.go` best-effort fanout — real HTTP POST for webhook/slack, log-only
      for email/pagerduty/telegram/whatsapp).
- [x] AI Incident Hub frontend (incident detail/RCA page) — `incident-hub/page.tsx`
      (+ sidebar NAV); list/ack/resolve via `alerts/ai_incidents.go` + `alerts/api.go`
      `/incidents` handler.
- [ ] Mobile alerts + AI screens

### Sprint 3 — ISP features
- [x] Sites management: `backend/internal/sites` (repository.go + api.go), migration
      0018 `sites` table; routes GET/POST `/api/v1/sites` + GET/PUT/DELETE
      `/api/v1/sites/{id}` (provisioning idiom, `r.PathValue("id")`); tenant scoping by
      optional `tenantId` query (session-scoping plumbing lands in Sprint 4).
- [x] AccessPoint CRUD: `backend/internal/accesspoints` (repository.go + api.go),
      migration 0018 `access_points`; routes `/api/v1/access-points` (+`/{id}`);
      linkable to sites/device; footprint JSONB.
- [x] CustomerConnection CRUD: `backend/internal/customers` (repository.go + api.go),
      migration 0018 `customer_connections`; routes `/api/v1/customers` (+`/{id}`);
      plan/bandwidth/contract fields.
- [x] Customers + sites frontend pages: `sites/page.tsx`, `access-points/page.tsx`,
      `customers/page.tsx` under `(noc)` + sidebar NAV (Sites ⌖ / Access Points ◬ /
      Customers ◉). Verified via `next build` (18 routes).
- [x] ConfigTemplate / provisioning extension: existing `provisioning_templates`
      already serve as ConfigTemplates; extended `RenderData` with `SerialNumber` +
      `Model` so RouterOS templates can reference `{{.SerialNumber}}`/`{{.Model}}`
      (PreviewAPI + FetchAPI). ⚠ static review passed; not yet `go build` verified.
- [ ] Mobile push notifications (waits on Expo mobile client)

### Sprint 4 — Hardening
- [ ] Multi-tenancy (Tenant model + API-key auth)
- [ ] Audit log middleware
- [ ] Redis caching + pgvector similarity for AI incidents
- [ ] Load test; mobile polish/release

### Gap map (prompt feature → current state)
- **AI incidents/RCA**: DONE (Sprint 2) but heuristic-only. `ai_incidents` (0015) is the
  durable store; `alerts/ai_incidents.go` `IncidentBridge` persists + runs `alerts/rca.go`
  deterministic analyzer + SSE. No external AI (offline): RCA/confidence are deterministic.
- **LLDP topology links**: DONE (Sprint 1). `backend/internal/topology` LLDP-MIB walk +
  graph model + scheduled Discovery engine (engine.go) persists links + snapshots
  (repository.go) and serves a persisted graph (`/api/topology`).
- **ICMP ping**: DONE (Sprint 0). `backend/internal/ping` poller + `/api/v1/ping/{id}/...`.
- **Generic alert rules**: DONE (Sprint 2). `alert_rules` (0016) persisted rules +
  `alerts/evaluator.go` loop over `metric_samples`, honoring `for_duration_sec`; dedup via
  in-memory `Engine`; API/UI (`alert-rules` page). SNMP trap rules and OLT optical alerts
  remain separate live surfaces.
- **Customers / access-points / sites / notification channels**: DONE (Sprint 3 + 2) —
  `sites`/`access_points`/`customer_connections` (0018) CRUD + pages; `notification_channels`
  (0017) fanout. Tenant-scoping is caller-supplied (`tenantId`/`organizationId` query) until
  Sprint 4 adds session-derived per-tenant auth.
- **Tenants / audit log / mobile / Redis+pgvector**: greenfield (Sprint 4).

## Conventions
- Backend: Go, `http.ServeMux` routes registered in `backend/cmd/api/main.go`;
  migrations are idempotent SQL numbered in `backend/migrations/`.
- Realtime is SSE (`/api/incidents/stream`) + polling on the frontend; no WebSocket.
- Config is env vars read in `main.go` (no config package, no `.env` loader).
- Frontend: `frontend/lib/api.ts` (`apiFetch` → relative `/api/v1/...`), per-page
  polling, no state store. Two URL conventions coexist (`/api/...` and `/api/v1/...`).
- Auth: httpOnly `routingnms_session` cookie; middleware checks presence only, backend enforces.
- Default admin: admin / admin123 (bootstrap in `internal/auth`).

## Verification
- `next build` works locally in `frontend/`.
- Go is NOT installed on the local dev machine — compile-verify Go via the Dockerfile
  (`deployments/` / `backend/Dockerfile`) and by matching existing Go patterns exactly.
