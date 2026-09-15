# Laundry Platform

Multi-outlet laundry management platform. Architecture is fully specified in
[`docs/`](docs) (Phase 0) — read `docs/PHASE-0-REVIEW.md` first for the
overall picture, then the numbered documents for detail. This root README
covers Phase 1: the technical foundation that runs against that spec.

## Status

- **Phase 0 (architecture):** complete. 15 documents in `docs/`, all
  cross-referenced and consistent (see `docs/PHASE-0-REVIEW.md` §5, §8).
- **Phase 1 (technical foundation, this scaffold):** monorepo, 5 Go
  services + Gateway, PostgreSQL migrations, RabbitMQ/Redis wiring, Docker
  Compose, CI, and a Next.js frontend shell. **No backend business
  logic/handlers yet** — every Go service exposes only `/healthz`,
  `/readyz`, `/metrics` today; the `TODO(phase-2)` comments in each
  `cmd/server/main.go` mark where `docs/07-api-contract.md`'s handlers
  attach next. The frontend's auth forms already call the correct Gateway
  endpoints and degrade gracefully until those handlers exist.

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

## Frontend (`web/`)

Next.js 16 (App Router, TypeScript, Tailwind v4). Brand palette and copy are
centralized in `web/lib/site.ts` and the `@theme` block in
`web/app/globals.css` — rename the brand there, not by hunting through
components. Structure:

```
web/app/                     routes: / (landing), /login, /register, /staff/login
web/components/ui/           brand-agnostic primitives (Button, Input, Container, WaveDivider, Logo)
web/components/layout/       Navbar, Footer, AuthLayout, WhatsAppButton
web/components/sections/     landing page sections (Hero, Services, HowItWorks, WhyUs, ...)
web/lib/                     site.ts (brand config), api.ts (Gateway fetch wrapper)
```

Auth forms call the Gateway directly (`/api/v1/auth/*` for staff,
`/api/v1/customer-auth/*` for customers per `docs/07-api-contract.md` §1–2)
and show a friendly error if it's unreachable — expected until Phase 2 lands.

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
