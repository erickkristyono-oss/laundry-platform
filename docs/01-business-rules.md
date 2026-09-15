# 01 — Business Rules

Status: **AUTHORITATIVE**. This document is the source of truth for business behavior. Every downstream document (workflow, service boundaries, schema, API, events, RBAC, state machines) must remain consistent with this file. Any conflict discovered later must be resolved by amending this file first.

---

## 1. Business Context

**Domain:** Multi-outlet laundry management platform (kilogram-based laundry service with pickup/delivery).

| Attribute | Value |
|---|---|
| Outlet model | Multi-outlet, single tenant (one business, many physical outlets) |
| Pricing model | Weight-based (per kg), global pricing at MVP |
| Fulfillment | In-store drop-off, and manually-coordinated pickup/delivery |
| Order channels | WEBSITE, POS, WHATSAPP |
| Customer types | Guest (no account) and Registered (saved profile) |
| Cashier/POS | Required (simple POS/cashier flow) |
| Loyalty/membership | Out of scope for MVP |
| Driver app | Out of scope for MVP — pickup/delivery is manually managed by staff |
| GPS tracking | Out of scope for MVP |

### 1.1 Order Sources

| Source | Description | MVP Integration Level |
|---|---|---|
| `WEBSITE` | Customer self-service order creation via Next.js web app | Full online flow (guest or registered) |
| `POS` | Staff-entered order at an outlet counter | Full, staff-operated |
| `WHATSAPP` | Order originating from a WhatsApp conversation | Staff-entered on the customer's behalf (see UQ-02 in Phase 0 Review — no automated WhatsApp Business API integration in MVP) |

---

## 2. Core Business Rules (Authoritative)

Each rule is assigned a stable ID (`BR-xx`) referenced by other documents. "Enforced at" indicates where the rule is defended in depth (a rule may be enforced at more than one layer).

| ID | Rule | Enforced at |
|---|---|---|
| BR-01 | One order may contain multiple services (line items). | Core DB (`order_items`), Core domain layer |
| BR-02 | Payment is performed **after** the laundry is weighed. | Core state machine (WEIGHING → WASHING gate), Payment Service |
| BR-03 | Estimated weight may be provided before receiving/weighing. | Core DB (`order_items.estimated_weight_kg`), API validation |
| BR-04 | Actual weight is determined by the outlet (staff), not the customer. | RBAC (LAUNDRY_STAFF/CASHIER only), Core domain layer |
| BR-05 | After payment is completed: actual weight, unit price, subtotal, and total amount become immutable. | Core DB check/trigger, Core domain layer, Payment Service event gate |
| BR-06 | Partial payment is **not** supported. | Payment Service domain layer, DB constraint (`payments.amount = orders.total_amount`) |
| BR-07 | Payment must cover the full order amount. | Payment Service domain layer |
| BR-08 | All outlets currently use global pricing. | Core DB (`service_prices.outlet_id IS NULL` for global rows) |
| BR-09 | The architecture must allow outlet-specific pricing later without major redesign. | Core DB schema design (`service_prices.outlet_id` nullable, resolution order: outlet-specific → global) |
| BR-10 | An order may be transferred from one outlet to another. | Core domain layer (`order_transfers`) |
| BR-11 | Every outlet transfer must be recorded in transfer history. | Core DB (`order_transfers`, append-only) |
| BR-12 | Customer may select an outlet manually. | API (`outlet_id` optional in order creation) |
| BR-13 | The system may recommend/auto-select an outlet based on coverage rules. | Core domain layer (`outlet_coverage_areas` lookup) |
| BR-14 | Pickup is manually managed (no driver app, no GPS). | Core DB (`pickup_requests`), operational process |
| BR-15 | Delivery is manually managed (no driver app, no GPS). | Core DB (`delivery_requests`), operational process |
| BR-16 | Customer may request cancellation/refund. | API (`POST /refunds`), Payment Service |
| BR-17 | Refund must be processed through an explicit refund workflow (state machine). | Payment Service domain layer |
| BR-18 | Customer must NOT be able to directly mark a payment as refunded. | RBAC (no customer-facing refund-approval permission), API authorization |
| BR-19 | Refund eligibility must be validated by the backend. | Payment Service domain layer, Core Service (order status check) |
| BR-20 | Order and payment data have a 1-year active retention period, then archive/retention — no immediate hard deletion. | Core/Payment DB (`archived_at` column + scheduled archival job), operational policy |
| BR-21 | Owner is GLOBAL and can access all outlets. | RBAC (`OWNER` scope = `GLOBAL`) |
| BR-22 | Outlet users are restricted according to their outlet scope. | RBAC (`user_outlets` mapping), API authorization middleware |
| BR-23 | Order and payment transactions must be auditable. | `order_status_history`, `payment_transactions`, `outbox_events`, structured audit log |
| BR-24 | Order/payment records must not be hard-deleted by normal application operations. | No `DELETE` endpoints exposed; soft-delete/archive only; DB privileges restrict `DELETE` on these tables from the application role |
| BR-25 | A registered customer may authenticate with a password-based login, independent of and never conferring any staff role. | Identity Service `customer_accounts`/`customer_sessions`; RBAC §"Customers" note in `09-rbac.md` |
| BR-26 | `OWNER`/`SUPER_ADMIN` may force `WEIGHING → WASHING` without payment for an explicit, audited, business reason (e.g. corporate net-terms accounts); the order must still reach `payment_status = PAID` before it can be marked `COMPLETED`. | `order_status_history.is_administrative_override`, `order.override_payment_gate` permission, `10-state-machines.md` §1.3 |
| BR-27 | PPN (VAT) is computed on every order's subtotal at a single, centrally-configured, versioned rate; changing the rate never alters previously finalized orders. | `tax_rates` (versioned, price-snapshot pattern), `orders.tax_amount`/`tax_rate_snapshot` |

BR-25 through BR-27 are not present in the Master Prompt's BR-001…BR-018 (they resolve UQ-03, UQ-05, and UQ-09 respectively, which the Master Prompt explicitly left as business confirmations rather than pre-decided rules — see `PHASE-0-REVIEW.md` §3). They carry the same authoritative weight as BR-01…BR-24 now that the business has confirmed them.

---

## 3. Assumptions Introduced at This Layer

These assumptions are required to make BR-01…BR-24 implementable. They are also logged in `PHASE-0-REVIEW.md` under Unresolved Questions.

- **A-01 (superseded by BR-25):** originally, "registered" customer meant a saved profile without necessarily implying password login. **Resolved (UQ-03):** password-based self-service login is now in scope — see BR-25 and `customer_accounts` in `06-database-schema.md` §1.9. A customer can still be "registered" (full profile, `is_guest=false`) without ever creating a login; the login is an additional, optional capability layered on top of the profile.
- **A-02:** `WHATSAPP` as an order source is operationally entered by staff (via POS-like interface) referencing the conversation; there is no automated WhatsApp Business API webhook integration in MVP.
- **A-03:** "Global pricing" means one active price per service across all outlets at any point in time; historical prices are versioned by effective date range, never overwritten.
- **A-04:** Retention (BR-20) is implemented as a status/flag-based archival (`is_archived`, `archived_at`) rather than moving rows to a separate physical archive store in MVP; physical cold storage can be introduced later without schema change to the active tables.

---

## 4. Business Rule ID Cross-Reference

A second, more consolidated Master Prompt for this same platform numbers business rules `BR-001`…`BR-018` instead of this document's `BR-01`…`BR-24`. Both numbering schemes describe the same 18 authoritative rule areas (this document simply keeps payment-integrity and refund sub-rules split into more granular, individually-testable IDs). No content conflict exists between the two; this table is the single mapping other documents can rely on rather than every cross-reference being duplicated under both schemes.

| Master Prompt ID | This document's ID(s) | Rule area |
|---|---|---|
| BR-001 | BR-01 | Multiple services per order |
| BR-002 | BR-03 | Estimated weight |
| BR-003 | BR-04 | Actual weight determined by outlet |
| BR-004 | BR-02 | Payment occurs after weighing |
| BR-005 | BR-06, BR-07 | No partial payment; full amount required |
| BR-006 | BR-05 | Weight/price/subtotal/total immutability after payment |
| BR-007 | BR-08, BR-09 | Global pricing today, outlet-specific-ready schema |
| BR-008 | (Phase 0 spec §8, enforced via BR-05) | Price snapshot |
| BR-009 | BR-10, BR-11 | Outlet transfer + history |
| BR-010 | BR-12, BR-13 | Manual or automatic outlet selection |
| BR-011 | BR-14 | Pickup, manually managed |
| BR-012 | BR-15 | Delivery, manually managed |
| BR-013 | (implied by BR-16, backend-validated) | Cancellation validated by backend |
| BR-014 | BR-16, BR-17, BR-18, BR-19 | Refund workflow, eligibility, no customer self-approval |
| BR-015 | BR-21 | OWNER is global |
| BR-016 | BR-20 | 1-year active retention, then archive |
| BR-017 | BR-23 | Auditability |
| BR-018 | BR-24 | No hard delete of financial records |

## 5. Rule Interaction Notes

- **BR-02 + BR-06 + BR-07** together mean the order pipeline has a hard synchronous-looking gate that is actually cross-service: Core Service owns the order state machine, but the transition out of `WEIGHING` is conditioned on a `payment.paid` event from Payment Service. See `10-state-machines.md` and `08-event-contract.md`.
- **BR-05** requires that once Payment Service emits `payment.paid` for an order, Core Service must reject any further mutation to that order's weight/price/subtotal/total at the API layer, in addition to Payment Service refusing a second payment attempt. This is a double-enforcement (defense in depth), not redundancy to be removed.
- **BR-08 + BR-09**: `service_prices` is modeled with a nullable `outlet_id`. Today, all rows have `outlet_id = NULL` (global). Enabling outlet-specific pricing later is purely a data change (insert outlet-scoped rows) with a documented resolution order (outlet-specific price wins over global price), not a schema migration.
- **BR-16 + BR-18 + BR-19**: Refund is a distinct aggregate/workflow, not a field flip on `payments`. The refund state machine (`10-state-machines.md`) is the only path to a `REFUNDED` payment state.
