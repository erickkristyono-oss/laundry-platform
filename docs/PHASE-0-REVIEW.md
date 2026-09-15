# PHASE 0 — Architecture Review

> This review has been updated to reconcile the initial Phase 0 spec with the subsequent, more detailed **MASTER PROMPT — PHASE 0: ARCHITECTURE & SYSTEM FOUNDATION**. The two are compatible; the Master Prompt is more prescriptive about a few points (a fully separate Payment state machine, optional order-level fee/discount/tax fields, a dedicated observability document, an additional ADR for Redis). All gaps identified between the two have been closed in this document set — see §5 "Contradictions" for how each was reconciled, and the per-document change notes referenced there.

## Document Set (15 files)

`01-business-rules.md`, `02-business-workflow.md`, `03-system-architecture.md`, `04-service-boundaries.md`, `05-database-erd.md`, `06-database-schema.md`, `07-api-contract.md`, `08-event-contract.md`, `09-rbac.md`, `10-state-machines.md`, `11-error-handling.md`, `12-security-baseline.md`, `13-observability.md`, `14-architecture-decisions.md`, `PHASE-0-REVIEW.md` (this file).

---

## 1. Decisions

- Five-service microservice architecture (Identity, Core, Payment, Notification, Reporting), monorepo, database-per-service, Transactional Outbox, RabbitMQ, REST `/api/v1`, PostgreSQL with UUID PKs + human-readable business codes, Redis for cache/rate-limiting/session lookups only (never a system of record). Full rationale in `14-architecture-decisions.md` (ADR-001 through ADR-012).
- Core Service intentionally kept coarse (customers, outlets, catalog/pricing, full order lifecycle) — not split further at this stage (Master Prompt §9.2).
- Payment Service isolated for compliance/integrity reasons, verifies order totals synchronously against Core rather than trusting client input.
- Order status is a 13-state, strictly sequential machine with `ON_HOLD` as a suspend/resume state and `CANCELLED` as terminal (`10-state-machines.md` §1).
- **Payment status is a separate 5-state machine** (`UNPAID → PENDING → PAID/FAILED → REFUNDED`), owned by Payment Service and merely projected read-only onto `orders.payment_status` in Core — satisfying the Master Prompt §8 requirement that payment state remain conceptually separate from order status (`10-state-machines.md` §2).
- Refund modeled as its own 6-state workflow, entirely separate from `payments.status`, with no direct customer-accessible mutation path.
- Pricing schema (`service_prices.outlet_id` nullable) supports global pricing today and outlet-specific pricing later with zero future migration.
- Price snapshot (`order_items.unit_price_snapshot`, `is_locked`) makes historical orders immune to future price changes.
- Order aggregate root includes `discount_amount`, `tax_amount`, `pickup_fee`, `delivery_fee` as schema-ready but **conditional/inert** fields (default `0`), per Master Prompt §17's instruction not to introduce mandatory tax/fee business rules that haven't been confirmed.
- RBAC: 5 roles, permission matrix with explicit outlet-scope column, server-side enforcement at two layers (Gateway + service).
- Observability baseline fixed as its own document (`13-observability.md`): structured logging, request ID, a distinct correlation ID for cross-service business operations, Prometheus metrics, liveness/readiness health checks, OpenTelemetry tracing.

## 2. Assumptions

| ID | Assumption | Recommended default applied |
|---|---|---|
| A-01 | *(superseded — see UQ-03 resolution below)* "Registered" customer = saved profile keyed by phone; not necessarily a password login | Core owns `customers` independently of Identity `users`; `identity_account_id` link is now **active** (business confirmed self-service login is required — see UQ-03) |
| A-02 | `WHATSAPP` orders are staff-entered, no WhatsApp Business API integration in MVP | `source` enum includes WHATSAPP as metadata only |
| A-03 | Global pricing = one active price per service at a time, versioned by effective date range | `service_prices` design in `06-database-schema.md` §2.6 |
| A-04 | Retention (BR-016/BR-20) implemented as flag-based archival, not physical cold storage, at MVP | `is_archived`/`archived_at` columns on `orders`/`payments` |
| A-05 | Business identifier format (sequential `-000001` vs. random `-xxxxxx` suffix) is an implementation detail, not a business rule — either satisfies "human-readable identifier, not a primary key" | `06-database-schema.md` §0 conventions; `14-architecture-decisions.md` ADR-006 |

## 3. Unresolved Questions

Per Master Prompt §34, each includes a recommended default, reason, and impact if changed later. **UQ-03, UQ-05, and UQ-09 have since been confirmed by the business owner** (2026-09-15) and are recorded here as resolved, since they affect API surface shape as flagged in §6; the other six remain open on their recommended defaults and do not block Phase 1.

| ID | Question | Status | Decision / Recommended Default | Reason | Impact if Changed Later |
|---|---|---|---|---|---|
| UQ-01 | What functionally distinguishes `SUPER_ADMIN` from `OWNER`? | Open (default) | `SUPER_ADMIN` = technical/platform administrator (system config, roles/permissions); `OWNER` = business owner (full business data, all outlets) | Both are listed without differentiation in either spec; this split gives each a distinct reason to exist rather than being redundant | Low — matrix in `09-rbac.md` can be split further later via permission-row edits, no schema change |
| UQ-02 | How are `WHATSAPP` orders actually captured — manual staff entry, or a future automated integration? | Open (default) | Manual staff entry via a POS-like flow in MVP | No WhatsApp Business API credentials/webhook infrastructure specified | Medium — automation later is an added Phase 1+ adapter component, not a redesign (source enum unaffected) |
| UQ-03 | Does "registered" customer require a password-based login/account, or just a saved profile? | **RESOLVED (2026-09-15)** | **Password-based self-service login IS required.** Identity Service gains `customer_accounts`/`customer_sessions` (see `06-database-schema.md` §1.9–1.10, `07-api-contract.md` §2, ADR-013); Core's `customers.identity_account_id` is now actively populated on registration. See BR-25. | Business confirmed customers need to view order history/status themselves, not just have a staff-recorded profile | N/A — resolved. Reversing later (dropping login) would be a scope reduction, not a redesign — the tables/endpoints would simply go unused. |
| UQ-04 | Refund eligibility window — time limit after `COMPLETED`? | Open (default) | 7 days post-completion (configurable) | No window specified in either spec | Low — pure parameter, no architectural impact |
| UQ-05 | Can `OWNER`/`SUPER_ADMIN` manually force `WEIGHING → WASHING` without payment (e.g., trusted corporate accounts)? | **RESOLVED (2026-09-15)** | **Override IS allowed**, restricted to `OWNER`/`SUPER_ADMIN`, mandatory `reason`, fully audited (`order_status_history.is_administrative_override`, `order.payment_overridden` event). `COMPLETED` still unconditionally requires `payment_status = PAID` — see `10-state-machines.md` §1.3, BR-26. | Business has trusted corporate/net-terms accounts that need processing to start before invoice payment clears | N/A — resolved. This was Phase 0's highest-rated risk (§4 risk 5); it is now an accepted, mitigated risk rather than an open question. |
| UQ-06 | Are partial refunds (less than full payment) allowed? | Open (default) | MVP treats all refunds as full-order refunds (BR-005/BR-06 keeps one payment per order, so no natural partial unit exists yet) | No partial-refund requirement stated | Medium — needs a refund-attribution concept on `order_items` if introduced later; Phase 1+ scoping question, not a Phase 0 blocker |
| UQ-07 | Service-to-service authentication: internal JWT vs. mTLS? | Open (default) | Deferred to Phase 1 — both compatible with the documented architecture | Pure implementation choice | Low — no schema/API contract impact |
| UQ-08 | Does Notification Service need to publish its own events later (e.g., `notification.failed`)? | Open (default) | Not in MVP scope; `outbox_events` table exists for platform consistency, expected to stay empty initially | No downstream consumer of Notification facts specified | Low |
| UQ-09 | Is there a confirmed tax (e.g., PPN) or pickup/delivery fee policy? | **PARTIALLY RESOLVED (2026-09-15)** | **PPN is confirmed and active**, but at a business-configured rate seeded at **0%** (turnover currently below the Rp 500 juta/year PKP threshold), computed on `subtotal_amount` only (pickup/delivery fees excluded from the tax base), stored versioned in `tax_rates` so the rate can change later without affecting historical orders — see `06-database-schema.md` §2.14, `07-api-contract.md` §7, ADR-014, BR-27. **Pickup fee and delivery fee remain unconfirmed** and stay at the original default (`0`, inert). | Master Prompt §17 forbids introducing tax/fees as mandatory rules without confirmation — tax is now confirmed, fees are not | Low — columns already exist; raising the PPN rate later (e.g. to 11%/12% once turnover crosses the threshold) is a single `PUT /api/v1/settings/tax-rate` call, not a schema or code change |

## 4. Risks

1. **Eventual consistency window** between Payment confirming `payment.paid` and Core reflecting `WASHING`/`orders.payment_status=PAID` — if the event relay/consumer lags, staff may see a transiently stale payment projection. Mitigated by a low outbox-relay poll interval and the `outbox_oldest_pending_age_seconds` alert defined in `13-observability.md` §8.
2. **Cross-service synchronous dependency** (Payment → Core for amount verification): if Core is down, no payment can be taken. Intentional trade-off (correctness over availability for money-handling); a cached last-known-total with a short TTL in Redis is a reasonable Phase 1 resilience addition, not required for Phase 0.
3. **Core Service breadth**: largest bounded context, most likely future candidate for decomposition if team/velocity outgrows it (ADR-010).
4. **Outbox relay as a new operational component**: if it silently stalls, writes still succeed but events stop flowing. Requires the health/lag metrics defined in `13-observability.md` §5/§8 from day one.
5. **UQ-05 (manual override) — RESOLVED, risk accepted with mitigation.** The business confirmed the override is needed (corporate/net-terms accounts). Rather than an undisciplined bypass, it is implemented as a narrowly-scoped, separately-audited exception: restricted to `OWNER`/`SUPER_ADMIN` only, mandatory `reason`, a dedicated `is_administrative_override` audit flag distinct from normal transitions, its own event (`order.payment_overridden`), and — critically — it does **not** waive the requirement that `payment_status = PAID` before `COMPLETED` (`10-state-machines.md` §1.2/§1.3). This preserves BR-02's money-safety property (every *completed* order was paid) while relaxing only the *timing* relative to washing for this one exception. Residual risk: an override could be left permanently unpaid if nobody follows up — mitigated operationally by the `order.payment_overridden` event feeding a Notification/Reporting "outstanding overrides" view (a Phase 1 UI concern, not an architectural gap).
6. **Conditional fee/tax fields (UQ-09)** — **PPN is now active** (seeded 0%), removing that portion of the risk entirely (it's no longer unused). `pickup_fee`/`delivery_fee` remain unconfirmed and inert; residual risk of confusion for Phase 1 engineers is unchanged and mitigated the same way — explicit "conditional" notes in `06-database-schema.md` §2.7 and the API contract's note that these are not client-supplied at order creation.
7. **New (UQ-03): customer credential compromise surface.** Adding customer self-service login introduces a new externally-reachable authentication surface (`customer_accounts`) that did not exist when Phase 0 was first reviewed. Mitigated by keeping it structurally isolated from staff `users`/`roles` (ADR-013 — a compromised customer account can never obtain a staff permission), and by applying the same password-hashing/rate-limiting/session-revocation baseline already specified for staff auth in `12-security-baseline.md`.

## 5. Contradictions

A full chain validation was performed per Master Prompt §33:

```
Business Rules → Business Workflow → Service Boundary → Database Boundary → ERD → API Contract → Event Contract → RBAC → Security
```

| Chain link | Checked against | Result |
|---|---|---|
| Business Rules → Workflow | Every rule (both `01-business-rules.md` BR-xx and the Master Prompt's BR-0xx, cross-referenced in `01-business-rules.md` §4) has a corresponding workflow step in `02-business-workflow.md` | ✅ Consistent |
| Workflow → Service Boundary | Every workflow step maps to exactly one owning service in `04-service-boundaries.md` §7 | ✅ Consistent |
| Service Boundary → Database Boundary | Every entity a service "owns" has a corresponding table in that service's logical DB in `06-database-schema.md`; no table appears in two services' schemas | ✅ Consistent |
| Database Boundary → ERD | Every table appears in the corresponding ERD in `05-database-erd.md`; every relationship is either an intra-service FK or an explicitly-labeled cross-service reference (no cross-service FK) | ✅ Consistent |
| ERD → API Contract | Every entity with a lifecycle has create/read/transition endpoints in `07-api-contract.md`; no endpoint references a field absent from the schema (including the newly-added `payment_status`, `discount_amount`, `tax_amount`, `pickup_fee`, `delivery_fee`) | ✅ Consistent |
| API Contract → Event Contract | Every cross-service-relevant state-changing endpoint emits an event listed in `08-event-contract.md` §3 | ✅ Consistent |
| Event Contract → RBAC | No event payload requires privileged access beyond what its consumers (trusted internal services) already have | ✅ Consistent |
| RBAC → Security | Every role/permission combination in `09-rbac.md` is enforceable given the AuthN/AuthZ mechanism in `12-security-baseline.md` | ✅ Consistent |

**No unresolved contradictions remain.** Points that required explicit reconciliation (not contradictions, but decisions that needed to be made consistently across documents) are recorded as resolved:

- **Payment vs. order status coupling:** `WEIGHING → WASHING` cannot be a purely Core-internal action because payment timing rules tie it to Payment Service's state. Resolved by making it a system-triggered transition consuming `payment.paid`, with `orders.payment_status` as an explicit, separate, eventually-consistent projection column rather than overloading `orders.status` — satisfying the Master Prompt's explicit requirement (§8) that the two state machines stay conceptually separate. Documented consistently in `10-state-machines.md`, `08-event-contract.md`, `06-database-schema.md` §2.7, and `04-service-boundaries.md`.
- **Refund's effect on order status:** resolved by treating `CANCELLED` as the single terminal non-success order state regardless of trigger (direct pre-payment cancellation vs. post-refund), with `order_status_history.reason` distinguishing the two for audit purposes — this also satisfies Master Prompt's example contradiction to check for ("actual weight can be modified after payment" and "refund can occur without authorization" are both explicitly prevented: see `06-database-schema.md` §2.8 `is_locked` and `09-rbac.md` §1's BR-18/refund.approve restriction to non-customer roles).
- **Documentation set drift between the two source prompts:** the original Phase 0 spec did not require a standalone observability document or a Redis ADR; the Master Prompt does. Resolved by adding `13-observability.md` and renumbering the ADR file to `14-architecture-decisions.md` with ADR-005 (Redis) inserted and Global Pricing renumbered to ADR-012, matching the Master Prompt's exact ADR list (§30) without discarding any previously-approved ADR content.

No instance was found of: API allowing an operation forbidden by business rules; RBAC permitting access outside outlet scope; the database allowing an invalid state (e.g., a `PAID`-and-later-modified `order_item` — prevented by `is_locked` plus the partial unique index in `06-database-schema.md` §3.1); an event payload lacking required information; payment occurring before weighing (prevented by the finalize-weighing precondition and payment's independent amount-verification read); a service accessing another service's database (Master Prompt §10's hard rule — verified against every table's "Service ownership" line in `06-database-schema.md`).

## 6. Recommendations

- None block Phase 1. **UQ-05, UQ-03, and UQ-09 (tax portion) have been confirmed** (2026-09-15) and are fully incorporated into `01-business-rules.md` (BR-25–27), `06-database-schema.md`, `07-api-contract.md`, `09-rbac.md`, `10-state-machines.md`, `08-event-contract.md`, `04-service-boundaries.md`, and `14-architecture-decisions.md` (ADR-013, ADR-014) — no remaining gap between this review and the rest of the document set.
- The remaining open items (UQ-01, UQ-02, UQ-04, UQ-06, UQ-07, UQ-08, and the pickup/delivery-fee portion of UQ-09) stay on their recommended defaults; none block Phase 1 infrastructure or handler work.
- Recommend Phase 1 introduce the internal service-to-service authentication mechanism (UQ-07) and the correlation-ID propagation plumbing (`13-observability.md` §3) together, early, since almost every other Phase 1 component (Gateway, all five services) depends on both from day one and retrofitting either later touches every service.

## 7. Phase 1 Prerequisites

Phase 1 ("Technical Foundation": monorepo, Go services, PostgreSQL, RabbitMQ, Redis, Docker, Gateway, CI/CD, Observability) can proceed with the following already fixed by Phase 0:

- Monorepo layout (`03-system-architecture.md` §3) — ready to scaffold.
- Go services boundaries and module split (`04-service-boundaries.md`) — ready to scaffold five Go modules + gateway.
- PostgreSQL schema, including the payment-status projection column and conditional fee/tax columns (`06-database-schema.md`) — ready to generate `sqlc` queries and migration files per service.
- RabbitMQ topology (exchanges/queues/routing keys) and the envelope's new `correlation_id` field (`08-event-contract.md` §1-2) — ready to configure.
- Redis usage patterns (session cache, rate limiting, price/outlet read-through cache) (`14-architecture-decisions.md` ADR-005, `12-security-baseline.md`) — ready to configure key namespaces per service.
- Docker Compose topology (`03-system-architecture.md` §5) — ready to write `docker-compose.yml`.
- Gateway routing table and auth-forwarding contract (`07-api-contract.md`, `12-security-baseline.md` §2) — ready to implement.
- Observability contract — structured log fields, request/correlation ID propagation, metric names, health-check endpoints, span naming (`13-observability.md`) — ready to instrument against.
- CI/CD: no architectural blocker; needs per-service build/test/lint pipelines plus a migration-check step — an engineering task, not a design decision.

Before Phase 1 begins, resolve (or explicitly accept the recommended defaults for) UQ-01 through UQ-09 above — none require re-architecture, but UQ-05, UQ-03, and UQ-09 affect API/handler surface and should be confirmed with the business owner first.

## 8. Completion Checklist (Master Prompt §35)

- [x] Business rules documented (`01-business-rules.md`, with cross-reference to the Master Prompt's BR-0xx numbering)
- [x] Business workflow documented (`02-business-workflow.md`)
- [x] Order state machine documented (`10-state-machines.md` §1)
- [x] Payment state machine documented, separate from order status (`10-state-machines.md` §2)
- [x] Refund workflow documented (`10-state-machines.md` §3)
- [x] Pickup workflow documented (`02-business-workflow.md` §4)
- [x] Delivery workflow documented (`02-business-workflow.md` §5)
- [x] Service boundaries documented (`04-service-boundaries.md`)
- [x] Database boundaries documented (`05-database-erd.md` §1, `06-database-schema.md` §0)
- [x] ERD completed (`05-database-erd.md`)
- [x] Table specifications completed (`06-database-schema.md`)
- [x] Index strategy completed (per-table indexes throughout `06-database-schema.md`)
- [x] API contracts completed (`07-api-contract.md`)
- [x] Event contracts completed (`08-event-contract.md`)
- [x] Outbox strategy completed (`06-database-schema.md` §6, `08-event-contract.md` §4)
- [x] Idempotency strategy completed (`11-error-handling.md` §4, `07-api-contract.md` §12)
- [x] RBAC completed (`09-rbac.md`)
- [x] Security baseline completed (`12-security-baseline.md`)
- [x] Observability baseline completed (`13-observability.md`)
- [x] ADRs completed (`14-architecture-decisions.md`, ADR-001 through ADR-012)
- [x] Architecture diagrams completed (distributed by topic per the index in `03-system-architecture.md` §6, plus the payment-state diagram in `10-state-machines.md` §2)
- [x] Cross-document consistency validated (§5 above)
- [x] No critical unresolved architectural contradiction remains (§5 above)

---

## 9. Final Status

```
PHASE 0 STATUS: READY FOR PHASE 1
```

Rationale: every item in the §8 completion checklist is satisfied, the full consistency chain (§5) shows no contradiction, and all nine open items (§3) are genuine business clarifications with safe, non-breaking recommended defaults — not architectural gaps. Coding, migrations, handlers, and infrastructure deployment have **not** been started, per the absolute rule in Master Prompt §37.

### PHASE 1 PREREQUISITES

See §7 above for the full list. In short: confirm or accept defaults for UQ-01–UQ-09 (business-owner input needed most for UQ-03, UQ-05, UQ-09), then Phase 1 may begin scaffolding the monorepo, the five Go services, PostgreSQL migrations, RabbitMQ/Redis configuration, the Gateway, Docker Compose, CI/CD pipelines, and OpenTelemetry/Prometheus/Grafana instrumentation — all against this document set as the source of truth.
