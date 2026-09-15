# Laundry Platform

Multi-outlet laundry management platform. Architecture is fully specified in
[`docs/`](docs) (Phase 0) — read `docs/PHASE-0-REVIEW.md` first for the
overall picture, then the numbered documents for detail. This root README
covers Phase 1: the technical foundation that runs against that spec.

## Status

- **Phase 0 (architecture):** complete. 15 documents in `docs/`, all
  cross-referenced and consistent (see `docs/PHASE-0-REVIEW.md` §5, §8).
- **Phase 1 (technical foundation):** complete. Monorepo, 5 Go services +
  Gateway, PostgreSQL migrations, RabbitMQ/Redis wiring, Docker Compose,
  CI, and a Next.js frontend shell.
- **Phase 2 (business logic), core flow — working end-to-end, in the
  browser, not just curl:** customer register/login, staff login, order
  creation (guest upsert-by-phone, outlet auto-recommend, price snapshot),
  receive → weigh → finalize weighing → cash payment (synchronously
  verified against Core) → event-driven `WEIGHING → WASHING` over
  RabbitMQ → staff-driven `DRYING → IRONING → PACKING → READY →
  PICKED_UP/DELIVERED → COMPLETED` (blocked unless paid, per BR-02).
  Idempotency-Key deduplication (docs/11-error-handling.md §4) is wired
  on register/order-creation/payment-creation. The frontend
  (customer dashboard + order form + status timeline, staff console with
  per-status actions) drives all of this directly against the Gateway.
  **Not yet built** (secondary features, each marked `TODO(phase-2+)` at
  its call site): refunds, pickup/delivery requests, order transfer, the
  UQ-05 administrative payment-gate override, staff user management, a
  key-expiry cleanup job, reporting projections, and notification
  rendering.

## Layout

```
services/{identity,core,payment,notification,reporting}/  one Go module each
gateway/                                                   API Gateway (Go module)
shared/                                                     cross-cutting plumbing only (see docs/03-system-architecture.md §4)
deployments/docker/                                         Dockerfiles + docker-compose.yml
docs/                                                        Phase 0 architecture (source of truth)
web/                                                         Next.js (App Router, TypeScript, Tailwind v4) customer/staff frontend
```

Each service is an independent Go module (own `go.mod`, own migrations,
own database) tied together for local development by `go.work`. `shared/`
is deliberately limited to business-rule-free plumbing (logging, HTTP
middleware, DB/Redis/RabbitMQ connection helpers, the outbox relay, the
migration runner) — see `docs/03-system-architecture.md` §4 for the exact
allow-list. No service imports another service's package.

## Running locally

```bash
make up      # docker compose up -d --build: Postgres, RabbitMQ, Redis, all 5 services, gateway
make down    # tear down
```

Each service applies its own migrations automatically on startup. Ports:

| Service | Container port | Host port (compose) |
|---|---|---|
| Gateway | 8080 | 8080 (the only one meant for client traffic) |
| Identity | 8081 | 18081 |
| Core | 8082 | 18082 |
| Payment | 8083 | 18083 |
| Notification | 8084 | 18084 |
| Reporting | 8085 | 18085 |
| Web | 3000 | 3000 |

Postgres (5432), RabbitMQ (5672, management UI 15672), Redis (6379) are also
exposed on their standard ports for local inspection.

On first boot against an empty database, Identity and Core each seed
themselves so there's something to log in against and order from — logged
loudly to stdout, local-dev only, never for a shared environment:
- Identity: one `OWNER` account, `owner@laundryku.local` / `ChangeMe123!`.
- Core: one outlet (`OUT-001`) and one service (`Cuci Kiloan`, Rp 7.000/kg).

Try the core flow with curl (Gateway at `localhost:8080`) or, easier, just
use the frontend below — both drive the same endpoints. Idempotency-Key
is required on register/order-creation/payment-creation:
```bash
curl -X POST localhost:8080/api/v1/customer-auth/register -H "Idempotency-Key: $(uuidgen)" -d '{"name":"Budi","phone":"0812xxxx","password":"password123"}'
curl -X POST localhost:8080/api/v1/auth/login -d '{"identifier":"owner@laundryku.local","password":"ChangeMe123!"}'
# then POST /api/v1/orders, /status (RECEIVED), PUT .../weigh, POST .../finalize-weighing, POST /api/v1/payments ...
```

## Frontend (`web/`)

Next.js 16 (App Router, TypeScript, Tailwind v4), talking directly to the
Gateway — no BFF layer. Brand palette and copy are centralized in
`web/lib/site.ts` and the `@theme` block in `web/app/globals.css` — rename
the brand there, not by hunting through components. Structure:

```
web/app/               marketing (/), customer auth (/login, /register), customer
                        (/dashboard, /order/new, /orders/[id]), staff (/staff/login,
                        /staff, /staff/orders/[id])
web/components/ui/     brand-agnostic primitives (Button, Input, Container, WaveDivider, Logo)
web/components/layout/ Navbar (session-aware via useSyncExternalStore), Footer, AuthLayout
web/components/order/  StatusBadge, StatusTimeline, OrderCard — shared by customer & staff views
web/lib/                site.ts (brand config), api.ts (Gateway fetch wrapper — attaches the
                        bearer token and an auto-generated Idempotency-Key), auth.ts (session
                        storage), useSession.ts (redirect-if-logged-out hooks)
```

Customer and staff sessions are independent (separate localStorage keys,
separate token audiences per ADR-013) — you can be logged in as both at
once in the same browser, e.g. to test the flow solo.

Auth forms call the Gateway directly (`/api/v1/auth/*` for staff,
`/api/v1/customer-auth/*` for customers per `docs/07-api-contract.md` §1–2)
against the real Identity endpoints (see Status above) and show a friendly
error if the Gateway is unreachable. Order/payment screens are not built yet.

```bash
cd web && cp .env.local.example .env.local && npm install && npm run dev
```

## Development

```bash
make build   # go build every module
make test    # go test every module
make fmt     # gofmt -l check
make vet     # go vet every module
make tidy    # go mod tidy every module

make migrate-up SERVICE=core     # apply one service's migrations standalone
make migrate-down SERVICE=core   # roll back its last migration
```

CI (`.github/workflows/ci.yml`) runs build/test/vet/fmt per module plus a
migration up/down check against a throwaway Postgres for every service.

## Where things are decided

- **Business rules, workflow, state machines, RBAC, event/API contracts:**
  `docs/`. If code and docs disagree, docs win — fix the code (or, if the
  business intent actually changed, update the doc set deliberately and
  note it, the way `PHASE-0-REVIEW.md` §3 tracks UQ-03/UQ-05/UQ-09).
- **Why a technology/pattern was chosen:** `docs/14-architecture-decisions.md`.
- **What each service owns and is allowed to call:**
  `docs/04-service-boundaries.md`.
