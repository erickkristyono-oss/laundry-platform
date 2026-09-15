# 06 — Database Schema Specification

## 0. Conventions

- **IDs:** every table has a `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` (requires `pgcrypto` or `pg_uuid` extension — `gen_random_uuid()` from `pgcrypto`).
- **Business identifiers:** human-readable, generated at insert time by application logic (not DB sequences, to keep them portable across future sharding): `CUS-xxxxxx`, `ORD-YYYYMMDD-xxxxxx`, `PAY-YYYYMMDD-xxxxxx`. Stored as `TEXT UNIQUE NOT NULL`, never used as a join key, never a primary key.
- **Timestamps:** `TIMESTAMPTZ NOT NULL DEFAULT now()` for `created_at`; `TIMESTAMPTZ` nullable for `updated_at` (updated via trigger or application) and lifecycle timestamps (`completed_at`, etc.).
- **Money:** `BIGINT` storing the smallest unit of IDR (Rupiah has no fractional subunit in practice, so `BIGINT` stores whole Rupiah). Never `FLOAT`/`DOUBLE`/`MONEY`.
- **Weight:** `NUMERIC(10,3)` (kilograms, 3 decimal places).
- **Soft delete / retention (BR-20, BR-24):** `is_archived BOOLEAN NOT NULL DEFAULT false`, `archived_at TIMESTAMPTZ`. No `DELETE` grants on transactional tables for the application DB role — only `SELECT, INSERT, UPDATE`.
- **Outbox tables:** identical shape across services — defined once in `10-outbox-pattern` conventions below, repeated per service schema.
- Every table lists **Service ownership** explicitly, even though it's implied by which logical database it lives in, to make cross-references unambiguous when read standalone.

---

## 1. IDENTITY SERVICE (`identity_db`)

### 1.1 `users`
Purpose: staff/admin login principals only. Customers never appear in this table, even after self-registering — a customer's login lives in the separate `customer_accounts` table (§1.9) precisely so a customer can never acquire a staff role via `user_roles` (see ADR-013 in `14-architecture-decisions.md`).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| email | TEXT | YES | — | UNIQUE (partial: WHERE email IS NOT NULL) |
| phone | TEXT | YES | — | UNIQUE (partial: WHERE phone IS NOT NULL) |
| full_name | TEXT | NO | — | |
| password_hash | TEXT | NO | — | argon2id hash, never plaintext |
| is_active | BOOLEAN | NO | true | |
| last_login_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |
| updated_at | TIMESTAMPTZ | YES | — | |

Constraints: `CHECK (email IS NOT NULL OR phone IS NOT NULL)`. Indexes: `idx_users_email`, `idx_users_phone`. Relationship: 1—N `user_roles`, `user_outlets`, `sessions`. Service ownership: **Identity**.

### 1.2 `roles`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| code | TEXT | NO | — | UNIQUE; one of SUPER_ADMIN, OWNER, OUTLET_ADMIN, CASHIER, LAUNDRY_STAFF |
| name | TEXT | NO | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Service ownership: **Identity**.

### 1.3 `permissions`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| code | TEXT | NO | — | UNIQUE, e.g. `order.create`, `refund.approve` |
| resource | TEXT | NO | — | e.g. `order` |
| action | TEXT | NO | — | e.g. `create`, `read`, `update`, `approve` |

Service ownership: **Identity**.

### 1.4 `user_roles`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| user_id | UUID | NO | — | FK → users.id ON DELETE CASCADE |
| role_id | UUID | NO | — | FK → roles.id ON DELETE RESTRICT |
| assigned_at | TIMESTAMPTZ | NO | now() | |

PK: `(user_id, role_id)`. Service ownership: **Identity**.

### 1.5 `role_permissions`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| role_id | UUID | NO | — | FK → roles.id ON DELETE CASCADE |
| permission_id | UUID | NO | — | FK → permissions.id ON DELETE RESTRICT |

PK: `(role_id, permission_id)`. Service ownership: **Identity**.

### 1.6 `user_outlets`
Purpose: outlet scope assignment (BR-21, BR-22).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| user_id | UUID | NO | — | FK → users.id ON DELETE CASCADE |
| outlet_id | UUID | NO | — | reference only — outlet lives in core_db, no FK |
| created_at | TIMESTAMPTZ | NO | now() | |

PK: `(user_id, outlet_id)`. Index: `idx_user_outlets_outlet_id`. Service ownership: **Identity**. Note: `OWNER`/`SUPER_ADMIN` roles have implicit GLOBAL scope and are not required to have rows here (absence of rows + role check = global access; see `09-rbac.md`).

### 1.7 `sessions`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| user_id | UUID | NO | — | FK → users.id ON DELETE CASCADE |
| refresh_token_hash | TEXT | NO | — | UNIQUE, hashed (never store raw token) |
| user_agent | TEXT | YES | — | |
| ip_address | INET | YES | — | |
| expires_at | TIMESTAMPTZ | NO | — | |
| revoked_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Indexes: `idx_sessions_user_id`, `idx_sessions_expires_at`. Service ownership: **Identity**.

### 1.8 `outbox_events` (identity_db)
See §6 "Outbox Table Standard" — identical shape, `service_name = 'identity'`.

### 1.9 `customer_accounts`
Purpose: **UQ-03 resolution** — customer self-service login credentials, deliberately kept separate from `users` (staff/RBAC principals) so a customer can never hold a staff role via `user_roles`. A customer's runtime identity is a *different principal type* from a staff user; JWTs issued to a customer carry `principal_type=CUSTOMER` and are never accepted by staff-only endpoints.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| customer_id | UUID | NO | — | reference only, no FK → `core_db.customers.id` (UNIQUE — one login per customer) |
| email | TEXT | YES | — | UNIQUE (partial: WHERE email IS NOT NULL) |
| phone | TEXT | YES | — | UNIQUE (partial: WHERE phone IS NOT NULL); login may use either identifier |
| password_hash | TEXT | NO | — | argon2id hash, never plaintext |
| email_verified_at | TIMESTAMPTZ | YES | — | |
| phone_verified_at | TIMESTAMPTZ | YES | — | |
| is_active | BOOLEAN | NO | true | |
| last_login_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |
| updated_at | TIMESTAMPTZ | YES | — | |

Constraints: `CHECK (email IS NOT NULL OR phone IS NOT NULL)`. Indexes: `idx_customer_accounts_customer_id` (unique), `idx_customer_accounts_email`, `idx_customer_accounts_phone`. Service ownership: **Identity**. Cross-service note: Core's `customers.identity_account_id` (§2.3) is the inverse logical reference; kept in sync via the customer registration flow (Identity issues the account, then calls Core — or consumes a `customer.registration_requested` internal call — to set the link; exact synchronous-vs-event mechanism is a Phase 1 implementation detail, not a Phase 0 architectural change since it is a single intra-request flow, not a new cross-service state machine).

### 1.10 `customer_sessions`
Purpose: refresh-token sessions for `customer_accounts`, mirroring `sessions` (§1.7) but kept in a distinct table rather than a polymorphic FK, so `sessions.user_id` can remain a clean, non-nullable FK to `users` for staff.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| customer_account_id | UUID | NO | — | FK → customer_accounts.id ON DELETE CASCADE |
| refresh_token_hash | TEXT | NO | — | UNIQUE, hashed (never store raw token) |
| user_agent | TEXT | YES | — | |
| ip_address | INET | YES | — | |
| expires_at | TIMESTAMPTZ | NO | — | |
| revoked_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Indexes: `idx_customer_sessions_account_id`, `idx_customer_sessions_expires_at`. Service ownership: **Identity**.

---

## 2. CORE SERVICE (`core_db`)

### 2.1 `outlets`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| code | TEXT | NO | — | UNIQUE |
| name | TEXT | NO | — | |
| address | TEXT | NO | — | |
| phone | TEXT | YES | — | |
| is_active | BOOLEAN | NO | true | |
| created_at | TIMESTAMPTZ | NO | now() | |

Service ownership: **Core**.

### 2.2 `outlet_coverage_areas`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| outlet_id | UUID | NO | — | FK → outlets.id ON DELETE CASCADE |
| area_label | TEXT | NO | — | e.g. district/kelurahan name |
| postal_code | TEXT | YES | — | |
| priority | INT | NO | 100 | lower = preferred when areas overlap |

Index: `idx_coverage_postal_code`. Unique: `(outlet_id, area_label)`. Service ownership: **Core**.

### 2.3 `customers`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| customer_code | TEXT | NO | — | UNIQUE, format `CUS-xxxxxx` |
| name | TEXT | NO | — | |
| phone | TEXT | NO | — | UNIQUE |
| email | TEXT | YES | — | |
| is_guest | BOOLEAN | NO | true | false once a full profile exists (staff-recorded) or the customer self-registers; independent of `identity_account_id` |
| identity_account_id | UUID | YES | — | reference only, no FK → `identity_db.customer_accounts.id` (UQ-03 resolved: populated once the customer creates a self-service login; NULL for guests and for staff-recorded profiles with no login) |
| created_at | TIMESTAMPTZ | NO | now() | |

Indexes: `idx_customers_phone`. Service ownership: **Core**.

### 2.4 `customer_addresses`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| customer_id | UUID | NO | — | FK → customers.id ON DELETE CASCADE |
| label | TEXT | NO | — | e.g. "Home", "Office" |
| address_line | TEXT | NO | — | |
| postal_code | TEXT | YES | — | |
| is_default | BOOLEAN | NO | false | |
| created_at | TIMESTAMPTZ | NO | now() | |

Index: `idx_customer_addresses_customer_id`. Service ownership: **Core**.

### 2.5 `services`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| code | TEXT | NO | — | UNIQUE |
| name | TEXT | NO | — | e.g. "Cuci Kering Setrika" |
| unit | TEXT | NO | 'kg' | CHECK (unit IN ('kg','pcs')) — kg is MVP default |
| is_active | BOOLEAN | NO | true | |
| created_at | TIMESTAMPTZ | NO | now() | |

Service ownership: **Core**.

### 2.6 `service_prices`
Purpose: versioned pricing; supports BR-08 (global today) and BR-09 (outlet-specific ready).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| service_id | UUID | NO | — | FK → services.id ON DELETE RESTRICT |
| outlet_id | UUID | YES | NULL | NULL = global price; non-null = outlet-specific override (future) |
| price_per_unit | BIGINT | NO | — | CHECK (price_per_unit > 0) |
| effective_from | TIMESTAMPTZ | NO | now() | |
| effective_to | TIMESTAMPTZ | YES | NULL | NULL = currently active |
| created_at | TIMESTAMPTZ | NO | now() | |

Constraint: exclusion constraint (or app-enforced) preventing overlapping `[effective_from, effective_to)` ranges per `(service_id, outlet_id)`. Index: `idx_service_prices_lookup (service_id, outlet_id, effective_from)`. Resolution order at read time: outlet-specific active row first, fallback to global (`outlet_id IS NULL`) active row. Service ownership: **Core**.

### 2.7 `orders`

Order is the aggregate root (Phase 0 spec §17). **UQ-09 resolved for tax**: `tax_amount`/`tax_rate_snapshot` are now **active** — PPN is enabled at a configurable rate (see `tax_rates`, §2.14), seeded at 0% (business is currently below the Rp 500 juta/year PKP threshold). `discount_amount`, `pickup_fee`, and `delivery_fee` remain **conditional**: schema-ready but not a confirmed business rule yet; they default to `0` and are inert until a business rule activates them, per Phase 0 spec §34 ("do not invent a major business rule").

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| order_code | TEXT | NO | — | UNIQUE, format `ORD-YYYYMMDD-xxxxxx` |
| customer_id | UUID | NO | — | FK → customers.id ON DELETE RESTRICT |
| current_outlet_id | UUID | NO | — | FK → outlets.id ON DELETE RESTRICT |
| source | TEXT | NO | — | CHECK (source IN ('WEBSITE','POS','WHATSAPP')) |
| fulfillment_type | TEXT | NO | 'WALK_IN' | CHECK (fulfillment_type IN ('WALK_IN','PICKUP','DELIVERY','PICKUP_AND_DELIVERY')) |
| status | TEXT | NO | 'CREATED' | order lifecycle — CHECK (status IN (13 statuses — see `10-state-machines.md` §1)) |
| payment_status | TEXT | NO | 'UNPAID' | **conceptually separate state machine** (Phase 0 spec §8) — CHECK (payment_status IN ('UNPAID','PENDING','PAID','FAILED','REFUNDED')); mirrors Payment Service's authoritative state, eventually consistent via `payment.*` events — see `10-state-machines.md` §2 |
| estimated_total_amount | BIGINT | YES | — | pre-weighing estimate, informational only |
| subtotal_amount | BIGINT | YES | — | sum of `order_items.subtotal`, authoritative once weighing finalized |
| discount_amount | BIGINT | NO | 0 | **conditional** — no discount business rule confirmed yet; CHECK (discount_amount >= 0) |
| tax_amount | BIGINT | NO | 0 | **active** (UQ-09 resolved) — computed at finalize-weighing time as `round(tax_rate_snapshot / 100 × subtotal_amount)`; basis is `subtotal_amount` only (pickup/delivery fees are NOT taxed); CHECK (tax_amount >= 0) |
| tax_rate_snapshot | NUMERIC(5,2) | NO | 0.00 | snapshot of the `tax_rates` row active at finalize-weighing time (price-snapshot pattern, BR-08) — makes historical orders immune to later rate changes; CHECK (tax_rate_snapshot >= 0) |
| pickup_fee | BIGINT | NO | 0 | **conditional** — flat/zero unless a pickup fee policy is confirmed; CHECK (pickup_fee >= 0) |
| delivery_fee | BIGINT | NO | 0 | **conditional** — flat/zero unless a delivery fee policy is confirmed; CHECK (delivery_fee >= 0) |
| total_amount | BIGINT | YES | — | = subtotal_amount − discount_amount + tax_amount + pickup_fee + delivery_fee; authoritative once weighing finalized; NOT NULL once status >= WEIGHING-finalized |
| notes | TEXT | YES | — | free-text staff/customer note |
| is_archived | BOOLEAN | NO | false | BR-20 |
| archived_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |
| updated_at | TIMESTAMPTZ | YES | — | |

Indexes: `idx_orders_customer_id`, `idx_orders_outlet_id`, `idx_orders_status`, `idx_orders_payment_status`, `idx_orders_created_at`. Check: `CHECK (total_amount IS NULL OR total_amount >= 0)`. Service ownership: **Core**.

> **Why `payment_status` lives on `orders` in `core_db` in addition to `payments.status` in `payment_db`:** Phase 0 spec §8 requires payment state to remain "conceptually separate" from order status, which this schema satisfies two ways at once — (1) it is a distinct column, not an overload of `status`, and (2) it is a distinct *state machine* owned by a distinct service. `orders.payment_status` is a read-optimized, eventually-consistent **projection** of Payment Service's authoritative `payments.status` (kept in sync by Core consuming `payment.created/paid/failed/refunded`), so the UI/API can filter/display payment state without a synchronous cross-service call on every order read. Payment Service's `payments.status` remains the single source of truth; `orders.payment_status` is never written by any path other than the event consumer.

### 2.8 `order_items`
Purpose: line items; carries the price snapshot (§8 of Phase 0 spec / BR-05, BR-006 in the Master Prompt numbering).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| order_id | UUID | NO | — | FK → orders.id ON DELETE CASCADE |
| service_id | UUID | NO | — | FK → services.id ON DELETE RESTRICT |
| service_name_snapshot | TEXT | NO | — | description snapshot (Phase 0 spec §18) — copy of `services.name` at order-creation time, so the line item remains readable even if the service catalog entry is later renamed |
| unit | TEXT | NO | — | copied from `services.unit` at creation time ('kg' or 'pcs') |
| estimated_weight_kg | NUMERIC(10,3) | YES | — | BR-03 (estimated quantity, kg-denominated at MVP; the same column serves a 'pcs' unit as a whole-number-valued NUMERIC when `unit='pcs'`) |
| actual_weight_kg | NUMERIC(10,3) | YES | — | BR-04; set only by staff during WEIGHING (actual quantity) |
| unit_price_snapshot | BIGINT | NO | — | copied from `service_prices` at the time weighing starts; immutable after payment (BR-05) |
| subtotal | BIGINT | YES | — | = actual_weight_kg * unit_price_snapshot, computed at finalize-weighing |
| is_locked | BOOLEAN | NO | false | set true once order payment succeeds (BR-05 enforcement) |
| created_at | TIMESTAMPTZ | NO | now() | |

Index: `idx_order_items_order_id`. Check: `CHECK (actual_weight_kg IS NULL OR actual_weight_kg >= 0)`. Application-layer trigger/guard: `UPDATE` on `actual_weight_kg`, `unit_price_snapshot`, `subtotal` rejected when `is_locked = true` (defense in depth alongside app logic — can be implemented as a `BEFORE UPDATE` trigger). Service ownership: **Core**.

### 2.9 `order_status_history`
Purpose: audit trail for every status transition (BR-23).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| order_id | UUID | NO | — | FK → orders.id ON DELETE CASCADE |
| from_status | TEXT | YES | — | NULL for the initial CREATED row |
| to_status | TEXT | NO | — | |
| reason | TEXT | YES | — | required for CANCELLED/ON_HOLD, and required when `is_administrative_override = true` |
| performed_by | UUID | NO | — | reference only, no FK (identity_db user) |
| is_administrative_override | BOOLEAN | NO | false | **UQ-05 resolution**: true only for the explicit exception where OWNER/SUPER_ADMIN forces `WEIGHING → WASHING` while `orders.payment_status <> 'PAID'` — see `10-state-machines.md` §1.3 |
| created_at | TIMESTAMPTZ | NO | now() | |

Index: `idx_order_status_history_order_id`, partial index `idx_order_status_history_override (order_id, created_at) WHERE is_administrative_override`. Append-only: no `UPDATE`/`DELETE` grants. Service ownership: **Core**.

### 2.10 `order_transfers`
Purpose: outlet transfer audit (BR-10, BR-11, §14 of Phase 0 spec).

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| order_id | UUID | NO | — | FK → orders.id ON DELETE CASCADE |
| from_outlet_id | UUID | NO | — | FK → outlets.id ON DELETE RESTRICT |
| to_outlet_id | UUID | NO | — | FK → outlets.id ON DELETE RESTRICT |
| reason | TEXT | NO | — | required, CHECK (length(reason) > 0) |
| performed_by | UUID | NO | — | reference only, no FK |
| created_at | TIMESTAMPTZ | NO | now() | |

Index: `idx_order_transfers_order_id`. Append-only. Service ownership: **Core**.

### 2.11 `pickup_requests`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| order_id | UUID | NO | — | FK → orders.id ON DELETE CASCADE, UNIQUE (one pickup request per order) |
| address_id | UUID | YES | — | FK → customer_addresses.id ON DELETE SET NULL |
| status | TEXT | NO | 'REQUESTED' | CHECK (status IN ('REQUESTED','SCHEDULED','COMPLETED','CANCELLED')) |
| scheduled_at | TIMESTAMPTZ | YES | — | |
| completed_at | TIMESTAMPTZ | YES | — | |
| notes | TEXT | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Service ownership: **Core**.

### 2.12 `delivery_requests`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| order_id | UUID | NO | — | FK → orders.id ON DELETE CASCADE, UNIQUE |
| address_id | UUID | YES | — | FK → customer_addresses.id ON DELETE SET NULL |
| status | TEXT | NO | 'REQUESTED' | CHECK (status IN ('REQUESTED','SCHEDULED','COMPLETED','CANCELLED')) |
| scheduled_at | TIMESTAMPTZ | YES | — | |
| completed_at | TIMESTAMPTZ | YES | — | |
| notes | TEXT | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Service ownership: **Core**.

### 2.13 `outbox_events` (core_db)
See §6. `service_name = 'core'`.

### 2.14 `tax_rates`
Purpose: **UQ-09 resolution** — versioned, admin-configurable PPN rate, following the exact same append-only versioning pattern as `service_prices` (§2.6) so historical orders keep an immutable `tax_rate_snapshot` (§2.7) regardless of later rate changes.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| rate_percentage | NUMERIC(5,2) | NO | — | CHECK (rate_percentage >= 0 AND rate_percentage <= 100) |
| effective_from | TIMESTAMPTZ | NO | now() | |
| effective_to | TIMESTAMPTZ | YES | NULL | NULL = currently active |
| reason | TEXT | NO | — | e.g. "Di bawah ambang batas PKP Rp 500 juta/tahun" or "Omset melewati ambang batas PKP, PPN diaktifkan" |
| created_by | UUID | NO | — | reference only, no FK (identity_db user; must hold `tax_settings.manage`, i.e. OWNER/SUPER_ADMIN — see `09-rbac.md`) |
| created_at | TIMESTAMPTZ | NO | now() | |

Constraint: partial unique index `idx_tax_rates_active UNIQUE (effective_to) WHERE effective_to IS NULL` — enforces at most one currently-active rate at a time (setting a new rate closes the previous row's `effective_to` in the same transaction). Index: `idx_tax_rates_effective (effective_from, effective_to)`. Seed data: one row, `rate_percentage = 0.00`, `reason = 'Di bawah ambang batas PKP Rp 500 juta/tahun'`. Service ownership: **Core**. Read pattern mirrors `service_prices`: order finalize-weighing reads the row where `effective_to IS NULL`, copies its `rate_percentage` into `orders.tax_rate_snapshot`, and computes `orders.tax_amount` from it — never re-reads the live rate afterward.

---

## 3. PAYMENT SERVICE (`payment_db`)

### 3.1 `payments`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| payment_code | TEXT | NO | — | UNIQUE, format `PAY-YYYYMMDD-xxxxxx` |
| order_id | UUID | NO | — | reference only, no FK; UNIQUE (one successful payment per order — BR-06) |
| customer_id | UUID | NO | — | reference only, no FK |
| amount | BIGINT | NO | — | CHECK (amount > 0) — must equal order total (BR-07), enforced in domain layer |
| method | TEXT | NO | — | CHECK (method IN ('CASH','QRIS','TRANSFER','CARD')) |
| status | TEXT | NO | 'PENDING' | CHECK (status IN ('PENDING','PAID','FAILED','REFUNDED')) |
| paid_at | TIMESTAMPTZ | YES | — | |
| is_archived | BOOLEAN | NO | false | BR-20 |
| archived_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Unique index: `uq_payments_order_id_status_paid` — a **partial unique index** `UNIQUE (order_id) WHERE status = 'PAID'` is the concrete mechanism guaranteeing BR-06 at the database layer (only one row per order can ever reach `PAID`). Service ownership: **Payment**.

### 3.2 `payment_transactions`
Purpose: append-only ledger of every attempt (BR-23 auditability), independent of the current `payments.status`.

| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| payment_id | UUID | NO | — | FK → payments.id ON DELETE CASCADE |
| transaction_type | TEXT | NO | — | CHECK (transaction_type IN ('CHARGE','CHARGE_FAILED','REFUND')) |
| amount | BIGINT | NO | — | |
| status | TEXT | NO | — | CHECK (status IN ('SUCCESS','FAILED','PENDING')) |
| gateway_response | JSONB | YES | — | raw gateway payload for audit |
| created_at | TIMESTAMPTZ | NO | now() | |

Index: `idx_payment_transactions_payment_id`. Append-only. Service ownership: **Payment**.

### 3.3 `refunds`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| refund_code | TEXT | NO | — | UNIQUE, format `RFD-YYYYMMDD-xxxxxx` |
| payment_id | UUID | NO | — | FK → payments.id ON DELETE RESTRICT |
| order_id | UUID | NO | — | reference only, no FK |
| amount | BIGINT | NO | — | CHECK (amount > 0) |
| status | TEXT | NO | 'REQUESTED' | CHECK (status IN ('REQUESTED','APPROVED','PROCESSING','COMPLETED','FAILED','REJECTED')) |
| reason | TEXT | NO | — | |
| requested_by | UUID | NO | — | reference only, no FK (may be a customer or staff user id) |
| approved_by | UUID | YES | — | reference only, no FK; required once status leaves REQUESTED via approval path |
| processed_at | TIMESTAMPTZ | YES | — | |
| created_at | TIMESTAMPTZ | NO | now() | |

Index: `idx_refunds_payment_id`. Check: refund `amount` must not exceed the associated payment's `amount` (enforced in domain layer at approval time, since it requires a cross-row read). Service ownership: **Payment**.

### 3.4 `outbox_events` (payment_db)
See §6. `service_name = 'payment'`.

---

## 4. NOTIFICATION SERVICE (`notification_db`)

### 4.1 `notifications`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| type | TEXT | NO | — | e.g. `ORDER_STATUS_CHANGED`, `PAYMENT_RECEIVED` |
| recipient_customer_id | UUID | YES | — | reference only, no FK |
| recipient_user_id | UUID | YES | — | reference only, no FK |
| source_event_id | UUID | NO | — | the domain event id that triggered this notification (idempotency key) |
| payload | JSONB | NO | — | rendered template variables |
| created_at | TIMESTAMPTZ | NO | now() | |

Unique: `uq_notifications_source_event_id` (idempotent event handling — see `08-event-contract.md` §4). Check: `CHECK (recipient_customer_id IS NOT NULL OR recipient_user_id IS NOT NULL)`. Service ownership: **Notification**.

### 4.2 `notification_deliveries`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK |
| notification_id | UUID | NO | — | FK → notifications.id ON DELETE CASCADE |
| channel | TEXT | NO | — | CHECK (channel IN ('SMS','WHATSAPP','EMAIL','PUSH')) |
| status | TEXT | NO | 'PENDING' | CHECK (status IN ('PENDING','SENT','FAILED')) |
| attempt_count | INT | NO | 0 | |
| last_attempt_at | TIMESTAMPTZ | YES | — | |
| error_message | TEXT | YES | — | |

Index: `idx_notification_deliveries_notification_id`. Service ownership: **Notification**.

### 4.3 `outbox_events` (notification_db)
See §6. `service_name = 'notification'`. (Present for platform consistency; MVP has no downstream consumers of Notification's own events.)

---

## 5. REPORTING SERVICE (`reporting_db`)

Read-only projections, rebuildable from the event log. No `outbox_events` table (Reporting publishes no events — see `04-service-boundaries.md` §6). Instead it has `projection_checkpoints` as its idempotency ledger.

### 5.1 `rpt_daily_outlet_sales`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| sales_date | DATE | NO | — | PK (composite) |
| outlet_id | UUID | NO | — | PK (composite), reference only |
| gross_amount | BIGINT | NO | 0 | |
| refunded_amount | BIGINT | NO | 0 | |
| order_count | INT | NO | 0 | |
| updated_at | TIMESTAMPTZ | NO | now() | |

Purpose: daily revenue per outlet dashboard. Service ownership: **Reporting**.

### 5.2 `rpt_order_summary`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| order_id | UUID | NO | — | PK, reference only |
| order_code | TEXT | NO | — | |
| outlet_id | UUID | NO | — | reference only |
| customer_id | UUID | NO | — | reference only |
| status | TEXT | NO | — | denormalized current status |
| total_amount | BIGINT | YES | — | |
| created_at | TIMESTAMPTZ | NO | — | |
| completed_at | TIMESTAMPTZ | YES | — | |

Purpose: flattened order list for fast reporting queries without hitting `core_db`. Service ownership: **Reporting**.

### 5.3 `rpt_service_performance`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| period_date | DATE | NO | — | PK (composite) |
| service_id | UUID | NO | — | PK (composite), reference only |
| total_weight_kg | NUMERIC(12,3) | NO | 0 | |
| total_revenue | BIGINT | NO | 0 | |

Service ownership: **Reporting**.

### 5.4 `rpt_refund_summary`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| period_date | DATE | NO | — | PK (composite) |
| outlet_id | UUID | NO | — | PK (composite), reference only |
| refund_count | INT | NO | 0 | |
| refund_amount | BIGINT | NO | 0 | |

Service ownership: **Reporting**.

### 5.5 `projection_checkpoints`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| projection_name | TEXT | NO | — | PK |
| last_event_id | UUID | YES | — | last successfully-applied event, for idempotent replay |
| last_processed_at | TIMESTAMPTZ | YES | — | |

Service ownership: **Reporting**.

---

## 6. Outbox Table Standard (repeated per service: identity, core, payment, notification)

### `outbox_events`
| Column | Type | Null | Default | Notes |
|---|---|---|---|---|
| id | UUID | NO | gen_random_uuid() | PK; equals `event_id` in the published envelope |
| aggregate_type | TEXT | NO | — | e.g. `order`, `payment` |
| aggregate_id | UUID | NO | — | the domain entity id |
| event_type | TEXT | NO | — | e.g. `order.created` |
| event_version | INT | NO | 1 | |
| payload | JSONB | NO | — | full event payload (see `08-event-contract.md`) |
| status | TEXT | NO | 'PENDING' | CHECK (status IN ('PENDING','PUBLISHING','PUBLISHED','FAILED')) — `PUBLISHING` is a short-lived claimed-but-not-yet-confirmed state used by the relay to atomically claim a batch (single `UPDATE ... RETURNING` with `SKIP LOCKED`) before making the network call, so two relay instances can never publish the same row twice; reverts to `PENDING` (with `retry_count` incremented) on publish failure |
| retry_count | INT | NO | 0 | |
| created_at | TIMESTAMPTZ | NO | now() | |
| published_at | TIMESTAMPTZ | YES | — | |

Indexes: `idx_outbox_status_created (status, created_at)` for the relay poller. Written in the **same DB transaction** as the domain state change (this is the entire point of the pattern — see `13-architecture-decisions.md` ADR-007). Full publishing/retry flow specified in `08-event-contract.md` §4.

---

## 7. Price Snapshot Enforcement Summary (Phase 0 spec §8)

1. At order-item creation, `unit_price_snapshot` is copied from the currently-active `service_prices` row (resolved per §2.6 rule) into `order_items.unit_price_snapshot`.
2. `order_items` never joins to `service_prices` at read time for historical orders — the UI/reports always read the stored snapshot.
3. Changing `service_prices` (inserting a new versioned row with a new `effective_from`) never mutates existing `order_items` rows — there is no `UPDATE ... FROM service_prices` path in the codebase, and application code is prohibited (by code review policy, documented here) from ever writing `service_prices.price_per_unit` into an existing row; new price = new row.
4. After payment succeeds, `order_items.is_locked = true` freezes `unit_price_snapshot`, `actual_weight_kg`, and `subtotal` (BR-05), enforced by a `BEFORE UPDATE` trigger in addition to application checks.
