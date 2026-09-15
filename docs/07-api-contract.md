# 07 — API Contract

Base path: `/api/v1`. Style: REST, JSON bodies, OpenAPI-compatible (a full `openapi.yaml` per service is a Phase 1 deliverable generated from this contract — not implemented in Phase 0). All endpoints go through the Gateway.

## 0. Conventions

- **Auth header:** `Authorization: Bearer <access_token>` unless marked `PUBLIC`.
- **Idempotency:** any `POST` that creates a financially or physically consequential resource (order, payment, refund) requires an `Idempotency-Key` header. See `11-error-handling.md` §4.
- **Pagination:** `?page=&page_size=` (default 20, max 100), response wrapped as `{ "data": [...], "meta": { "page", "page_size", "total" } }`.
- **Error shape:** see `11-error-handling.md` §1.
- **Money fields:** integers (IDR, whole Rupiah). **Weight fields:** decimal strings with 3 dp.
- Roles referenced below are defined in `09-rbac.md`.

---

## 1. Authentication (`Identity Service`)

### `POST /api/v1/auth/login`
- Auth: PUBLIC. Authz: none.
- Request: `{ "identifier": "email or phone", "password": "string" }`
- Response `200`: `{ "access_token", "refresh_token", "expires_in", "user": { "id","full_name","roles":[...] } }`
- Validation: identifier required; password required.
- Errors: `401 INVALID_CREDENTIALS`, `423 ACCOUNT_INACTIVE`, `429 RATE_LIMITED`.
- Idempotency: not required (read-mostly, side effect is a new session row — duplicate logins are harmless).

### `POST /api/v1/auth/refresh`
- Auth: refresh token in body. Authz: none.
- Request: `{ "refresh_token": "string" }` → Response `200`: new `access_token`/`refresh_token` pair (rotation).
- Errors: `401 INVALID_TOKEN`, `401 TOKEN_REVOKED`.

### `POST /api/v1/auth/logout`
- Auth: required. Authz: self.
- Request: `{ "refresh_token": "string" }` → Response `204`.
- Effect: revokes the session row.

### `GET /api/v1/auth/me`
- Auth: required.
- Response `200`: current user profile + roles + outlet scopes.

---

## 2. Customer Authentication (`Identity Service`) — **UQ-03 resolution**

Distinct from §1: issues **customer-scoped** tokens (`principal_type=CUSTOMER`, `customer_accounts`/`customer_sessions` — see `06-database-schema.md` §1.9–1.10), never accepted by staff-only endpoints, and never carries any `09-rbac.md` staff role.

### `POST /api/v1/customer-auth/register`
- Auth: PUBLIC. Authz: none.
- Request: `{ "name","email?","phone?","password" }` — at least one of email/phone required.
- Effect: creates (or upgrades an existing guest) `customers` row in Core, creates the `customer_accounts` row in Identity, sets `customers.identity_account_id` and `customers.is_guest = false`.
- Response `201`: `{ "access_token","refresh_token","expires_in","customer": {"id","name","customer_code"} }`.
- Errors: `409 EMAIL_ALREADY_EXISTS` / `409 PHONE_ALREADY_EXISTS`, `422 VALIDATION_ERROR` (e.g. weak password).
- Idempotency: required (`Idempotency-Key`).

### `POST /api/v1/customer-auth/login`
- Auth: PUBLIC.
- Request: `{ "identifier": "email or phone", "password" }` → Response `200`: same shape as register's response.
- Errors: `401 INVALID_CREDENTIALS`, `423 ACCOUNT_INACTIVE`, `429 RATE_LIMITED`.

### `POST /api/v1/customer-auth/refresh`
- Request: `{ "refresh_token" }` → Response `200`: new token pair (rotation). Errors: `401 INVALID_TOKEN`, `401 TOKEN_REVOKED`.

### `POST /api/v1/customer-auth/logout`
- Auth: required (customer token). Request: `{ "refresh_token" }` → Response `204`.

### `GET /api/v1/customer-auth/me`
- Auth: required (customer token). Response `200`: customer profile + addresses.

### `POST /api/v1/customer-auth/password-reset/request`
- Auth: PUBLIC. Request: `{ "identifier": "email or phone" }` → Response `202` always (never reveals whether the identifier exists — enumeration protection). Effect: emits a reset token via Notification (email/WhatsApp), TTL 15 minutes, single-use, stored hashed.

### `POST /api/v1/customer-auth/password-reset/confirm`
- Auth: PUBLIC. Request: `{ "reset_token","new_password" }` → Response `204`. Effect: invalidates all existing `customer_sessions` for that account (force re-login everywhere). Errors: `400 INVALID_OR_EXPIRED_TOKEN`.

---

## 3. Users (`Identity Service`)

### `POST /api/v1/users`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`.
- Request: `{ "full_name","email?","phone?","password","role_codes":[...], "outlet_ids":[...] }`
- Response `201`: user object.
- Validation: at least one of email/phone; role_codes non-empty and must exist; `outlet_ids` required unless role is global (OWNER/SUPER_ADMIN).
- Errors: `409 EMAIL_ALREADY_EXISTS`, `422 VALIDATION_ERROR`.
- Idempotency: required (`Idempotency-Key`).

### `GET /api/v1/users` / `GET /api/v1/users/{id}`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`, `OUTLET_ADMIN` (scoped to own outlet's staff only).
- Response: user list/detail with roles and outlet scopes.

### `PATCH /api/v1/users/{id}`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`.
- Request: partial update (`is_active`, `role_codes`, `outlet_ids`).
- Errors: `404 NOT_FOUND`, `422 VALIDATION_ERROR`.

---

## 4. Customers (`Core Service`)

### `POST /api/v1/customers`
- Auth: PUBLIC (guest creation) or required (staff creating on behalf).
- Request: `{ "name","phone","email?" }`
- Response `201`: `{ "id","customer_code","is_guest": true }`.
- Validation: phone format (Indonesian mobile), uniqueness by phone (if exists, returns existing customer instead of erroring — upsert-by-phone semantics for guest flow).
- Idempotency: required.

### `GET /api/v1/customers/{id}`
- Auth: required. Authz: `CASHIER+` scoped to outlet-of-order association, or `OWNER`/`SUPER_ADMIN` global.

### `POST /api/v1/customers/{id}/addresses`
- Auth: required (self via customer session, or staff).
- Request: `{ "label","address_line","postal_code?","is_default?" }`.

---

## 5. Outlets (`Core Service`)

### `GET /api/v1/outlets`
- Auth: PUBLIC.
- Response: active outlet list (for website outlet picker).

### `POST /api/v1/outlets`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`.
- Request: `{ "code","name","address","phone?" }`.
- Idempotency: required.

### `POST /api/v1/outlets/{id}/coverage-areas`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`, `OUTLET_ADMIN` (own outlet).
- Request: `{ "area_label","postal_code?","priority?" }`.

### `GET /api/v1/outlets/recommend?postal_code=&area_label=`
- Auth: PUBLIC.
- Response: recommended `outlet_id` based on coverage rules (BR-13), or `204 NO_MATCH` if none found (caller falls back to manual selection).

---

## 6. Services & Pricing (`Core Service`)

### `GET /api/v1/services`
- Auth: PUBLIC. Response: active service catalog with current effective price.

### `POST /api/v1/services`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`.
- Request: `{ "code","name","unit" }`.

### `POST /api/v1/pricing`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER`.
- Request: `{ "service_id","outlet_id?","price_per_unit","effective_from" }`.
- Validation: `price_per_unit > 0`; if `outlet_id` provided, must reference an existing outlet (future-ready field, optional in MVP UI).
- Effect: inserts a new `service_prices` row (never updates an existing one — see `06-database-schema.md` §7).

### `GET /api/v1/pricing?service_id=&outlet_id=`
- Auth: PUBLIC (needed for the order estimate UI).
- Response: currently active price, resolved outlet-specific-first-then-global.

---

## 7. Tax Settings (`Core Service`) — **UQ-09 resolution**

### `GET /api/v1/settings/tax-rate`
- Auth: PUBLIC (needed for the order estimate UI to show tax-inclusive totals).
- Response `200`: `{ "rate_percentage": "0.00", "effective_from": "2026-09-15T00:00:00Z" }` — the currently active `tax_rates` row (`effective_to IS NULL`).

### `PUT /api/v1/settings/tax-rate`
- Auth: required. Authz: `SUPER_ADMIN`, `OWNER` only (`tax_settings.manage`).
- Request: `{ "rate_percentage": "11.00", "reason": "Omset melewati ambang batas PKP, PPN diaktifkan" }`.
- Validation: `0 <= rate_percentage <= 100`; `reason` required (non-empty).
- Effect: closes the current active row's `effective_to = now()` and inserts a new active row, atomically (single DB transaction). **Does not retroactively change** `tax_amount`/`tax_rate_snapshot` on existing orders (price-snapshot pattern, BR-08) — only orders whose finalize-weighing happens after this call use the new rate.
- Response `201`: the new `tax_rates` row.
- Errors: `403 FORBIDDEN`, `422 VALIDATION_ERROR`.
- Idempotency: required (`Idempotency-Key`) — a duplicate submit must not create two rate-change rows.

---

## 8. Orders (`Core Service`)

### `POST /api/v1/orders`
- Auth: PUBLIC (guest) or required (registered/staff).
- Authz: any authenticated role may create; guests allowed for `source=WEBSITE`.
- Request:
```json
{
  "customer": { "id": "uuid | null", "name": "string (if new guest)", "phone": "string (if new guest)" },
  "outlet_id": "uuid | null (null = auto-recommend)",
  "source": "WEBSITE | POS | WHATSAPP",
  "fulfillment_type": "WALK_IN | PICKUP | DELIVERY | PICKUP_AND_DELIVERY",
  "items": [ { "service_id": "uuid", "estimated_weight_kg": "number | null" } ],
  "pickup_requested": "boolean",
  "delivery_requested": "boolean",
  "discount_amount": "integer | null (conditional — omit unless a discount policy is confirmed; defaults to 0)",
  "notes": "string | null"
}
```
- Response `201`: order object with `status=CREATED`, `payment_status=UNPAID`, `order_code`. The order object always includes `subtotal_amount`, `discount_amount`, `tax_amount`, `tax_rate_snapshot`, `pickup_fee`, `delivery_fee`, `total_amount` (all `0`/`null` as appropriate until weighing is finalized — see `06-database-schema.md` §2.7). `tax_amount`/`tax_rate_snapshot` are server-computed at finalize-weighing time from the active `tax_rates` row (**UQ-09 resolved** — mechanism active, seeded at 0%); `pickup_fee`/`delivery_fee` remain deferred (no confirmed fee policy — see `PHASE-0-REVIEW.md` UQ-09). None of these are client-supplied at order creation.
- Validation: `items` non-empty; `source` enum; if `outlet_id` is null, resolve via `GET /outlets/recommend`, else `422 NO_OUTLET_AVAILABLE`.
- Errors: `422 VALIDATION_ERROR`, `409 CUSTOMER_CONFLICT`.
- Idempotency: **required** (`Idempotency-Key`) — prevents duplicate order creation on client retry.

### `GET /api/v1/orders/{id}` / `GET /api/v1/orders?customer_id=&outlet_id=&status=&payment_status=`
- Auth: required. Authz: outlet-scoped for staff roles; customer may fetch own orders only.
- `status` filters on the order lifecycle state machine; `payment_status` filters independently on the payment state machine (`10-state-machines.md` §1 vs §2) — the two are never conflated into one filter parameter.

### `PUT /api/v1/orders/{id}/items/{itemId}/weigh`
- Auth: required. Authz: `CASHIER`, `LAUNDRY_STAFF`, `OUTLET_ADMIN` (own outlet).
- Request: `{ "actual_weight_kg": "number" }`.
- Precondition: order status must be `RECEIVED` or `WEIGHING`; item must not be `is_locked`.
- Effect: order transitions to/stays `WEIGHING`; recalculates item `subtotal`.
- Errors: `409 INVALID_STATE`, `409 ITEM_LOCKED`, `422 VALIDATION_ERROR`.

### `POST /api/v1/orders/{id}/finalize-weighing`
- Auth: required. Authz: `CASHIER`, `LAUNDRY_STAFF`, `OUTLET_ADMIN`.
- Precondition: all items have `actual_weight_kg` set.
- Request (optional body): `{ "pickup_fee": "integer | null", "delivery_fee": "integer | null" }` — only accepted if `fulfillment_type` includes pickup/delivery; both default to `0` (no confirmed fee policy in Phase 0 — see UQ-09). `tax_amount` and `tax_rate_snapshot` are never client-settable; both are server-computed from the currently active `tax_rates` row (**UQ-09 resolved**: mechanism is active, seeded at 0%).
- Effect: computes `orders.subtotal_amount` (sum of item subtotals), reads the active `tax_rates` row into `tax_rate_snapshot`, computes `tax_amount = round(tax_rate_snapshot/100 × subtotal_amount)`, then `orders.total_amount = subtotal_amount − discount_amount + tax_amount + pickup_fee + delivery_fee`; order remains `WEIGHING`, `payment_status` remains `UNPAID`, awaiting payment.
- Errors: `409 INCOMPLETE_WEIGHING`.

### `POST /api/v1/orders/{id}/status`
- Auth: required. Authz: role depends on target status — see `10-state-machines.md` transition table.
- Request: `{ "to_status": "string", "reason": "string (required for ON_HOLD/CANCELLED)" }`.
- Errors: `409 INVALID_TRANSITION`, `403 FORBIDDEN_TRANSITION`.
- Idempotency: required.
- Note: this endpoint does **not** accept `to_status=WASHING` from `WEIGHING` for any caller — that transition is either system-triggered by `payment.paid`, or performed exclusively via the dedicated override endpoint below (**UQ-05 resolution**), never via this generic endpoint, so the two paths can never be confused in an audit log.

### `POST /api/v1/orders/{id}/status/override` — **UQ-05 resolution**
- Auth: required. Authz: `OWNER`, `SUPER_ADMIN` only (`order.override_payment_gate`).
- Precondition: order is currently `WEIGHING` with `total_amount` already computed (finalize-weighing done).
- Request: `{ "reason": "string (required, min 10 chars)" }`.
- Effect: transitions the order directly to `WASHING` without requiring `payment_status = PAID`; writes `order_status_history` with `is_administrative_override = true`; emits `order.status_changed` **and** `order.payment_overridden` (see `08-event-contract.md`). `payment_status` is left untouched. Downstream, `{PICKED_UP, DELIVERED} → COMPLETED` still requires `payment_status = PAID` (see `10-state-machines.md` §1.2/§1.3).
- Errors: `403 FORBIDDEN` (role), `400 REASON_REQUIRED`, `409 INVALID_STATE` (order not in `WEIGHING` or weighing not finalized).
- Idempotency: required (`Idempotency-Key`).

### `POST /api/v1/orders/{id}/transfer`
- Auth: required. Authz: `OUTLET_ADMIN` (source outlet), `OWNER`, `SUPER_ADMIN`.
- Request: `{ "to_outlet_id","reason" }`.
- Precondition: order status not in `{PICKED_UP, DELIVERED, COMPLETED, CANCELLED}`.
- Effect: writes `order_transfers` row, updates `current_outlet_id`; **status unchanged** (see BR-10/BR-11).
- Errors: `409 INVALID_STATE`, `422 SAME_OUTLET`.
- Idempotency: required.

---

## 9. Pickup (`Core Service`)

### `POST /api/v1/orders/{id}/pickup`
- Auth: required or public-at-order-creation-time (embedded in order creation payload as `pickup_requested`; this endpoint covers post-creation requests).
- Request: `{ "address_id","preferred_window?" }`.
- Response `201`: pickup_request `status=REQUESTED`.

### `PATCH /api/v1/pickups/{id}`
- Auth: required. Authz: `CASHIER`, `OUTLET_ADMIN`, `LAUNDRY_STAFF`.
- Request: `{ "status": "SCHEDULED|COMPLETED|CANCELLED", "scheduled_at?" }`.
- Effect: `COMPLETED` triggers order status → `RECEIVED` (if not already past it).

---

## 10. Delivery (`Core Service`)

### `POST /api/v1/orders/{id}/delivery`
- Auth: required. Authz: `CASHIER`, `OUTLET_ADMIN`.
- Request: `{ "address_id","preferred_window?" }`.
- Precondition: order status = `READY`.

### `PATCH /api/v1/deliveries/{id}`
- Auth: required. Authz: `CASHIER`, `OUTLET_ADMIN`, `LAUNDRY_STAFF`.
- Request: `{ "status": "SCHEDULED|COMPLETED|CANCELLED", "scheduled_at?" }`.
- Effect: `COMPLETED` triggers order status → `DELIVERED`.

---

## 11. Payment (`Payment Service`)

### `POST /api/v1/payments`
- Auth: required. Authz: `CASHIER`, `OUTLET_ADMIN`, or the owning customer (self-checkout on website).
- Request: `{ "order_id","amount","method" }`.
- Validation: synchronously verify `amount == Core.orders.total_amount` (service-to-service read); order status must be `WEIGHING` with weighing finalized.
- Response `201`: payment object `status=PENDING` → (for CASH, immediately `PAID`; for QRIS/TRANSFER/CARD, `PAID` after gateway callback in Phase 1+, out of scope for Phase 0 wiring).
- Errors: `409 AMOUNT_MISMATCH` (BR-07), `409 ORDER_ALREADY_PAID` (BR-06), `409 INVALID_ORDER_STATE`.
- Idempotency: **required** — critical for BR-06 (prevents double-charge on retry).

### `GET /api/v1/payments/{id}` / `GET /api/v1/payments?order_id=`
- Auth: required. Authz: outlet-scoped / owning customer.

---

## 12. Refund (`Payment Service`)

### `POST /api/v1/refunds`
- Auth: required. Authz: owning customer, or `CASHIER`/`OUTLET_ADMIN` on behalf of customer.
- Request: `{ "payment_id","reason" }`.
- Effect: backend computes eligibility (BR-19) from order status (via Core sync read) and payment status; creates `refunds` row `status=REQUESTED`.
- Errors: `409 REFUND_NOT_ELIGIBLE`, `404 PAYMENT_NOT_FOUND`.
- Idempotency: required.

### `POST /api/v1/refunds/{id}/approve`
- Auth: required. Authz: `OUTLET_ADMIN`, `OWNER`, `SUPER_ADMIN`. **Never** the customer (BR-18).
- Request: `{ "amount?": "override, defaults to full payment amount" }`.
- Effect: `REQUESTED → APPROVED`, then service triggers `PROCESSING` automatically.

### `POST /api/v1/refunds/{id}/reject`
- Auth: required. Authz: `OUTLET_ADMIN`, `OWNER`, `SUPER_ADMIN`.
- Request: `{ "reason" }`.
- Effect: `REQUESTED → REJECTED` (terminal).

### `GET /api/v1/refunds/{id}` / `GET /api/v1/refunds?order_id=`
- Auth: required. Authz: outlet-scoped / owning customer (read-only).

---

## 13. Reports (`Reporting Service`, read-only)

### `GET /api/v1/reports/sales?from=&to=&outlet_id=`
- Auth: required. Authz: `OUTLET_ADMIN` (own outlet), `OWNER`, `SUPER_ADMIN` (all outlets).
- Response: daily sales series from `rpt_daily_outlet_sales`.

### `GET /api/v1/reports/orders/summary?from=&to=&outlet_id=&status=`
- Auth: required. Authz: as above.

### `GET /api/v1/reports/services/performance?from=&to=`
- Auth: required. Authz: `OWNER`, `SUPER_ADMIN`.

### `GET /api/v1/reports/refunds?from=&to=&outlet_id=`
- Auth: required. Authz: `OUTLET_ADMIN` (own outlet), `OWNER`, `SUPER_ADMIN`.

---

## 14. Idempotency Requirements Summary

| Endpoint class | Idempotency-Key required? | Key scope |
|---|---|---|
| `POST /orders` | Yes | per client-generated key, 24h dedup window |
| `POST /orders/{id}/status` | Yes | per key, 24h |
| `POST /orders/{id}/status/override` | Yes | per key, 24h |
| `POST /orders/{id}/transfer` | Yes | per key, 24h |
| `POST /customer-auth/register` | Yes | per key, 24h (also unique-by-email/phone as secondary guard) |
| `PUT /settings/tax-rate` | Yes | per key, 24h |
| `POST /payments` | Yes (critical) | per key, 24h, additionally guarded by DB partial-unique index on `order_id WHERE status='PAID'` |
| `POST /refunds`, `/approve`, `/reject` | Yes | per key, 24h |
| `POST /customers` | Yes | per key, 24h (also upsert-by-phone as secondary guard) |
| All `GET` | N/A (safe/idempotent by definition) | — |

Full mechanics (storage of key → response, replay behavior) in `11-error-handling.md` §4.
