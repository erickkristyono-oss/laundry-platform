# 09 — RBAC (Role-Based Access Control)

## 1. Roles

| Role | Scope | Description |
|---|---|---|
| `SUPER_ADMIN` | GLOBAL | Platform/technical administrator. Full access including system configuration (roles, permissions, service catalog structure). Intended for internal technical operators, not the business owner. |
| `OWNER` | GLOBAL | Business owner. Full access to business data across all outlets (orders, payments, refunds, reports, pricing). May not need raw system-configuration screens but is not blocked from them. |
| `OUTLET_ADMIN` | Assigned outlet(s) | Manages one or more specific outlets: staff, pricing visibility, order oversight, transfer initiation, refund approval for their outlet. |
| `CASHIER` | Assigned outlet(s) | Front-desk operations: create orders, record weighing, take payment, request pickup/delivery. |
| `LAUNDRY_STAFF` | Assigned outlet(s) | Back-of-house operations: record weighing, progress order status through WASHING→READY, mark pickup/delivery completion. |

> **Note (see PHASE-0-REVIEW.md, UQ-01):** the spec does not define the precise functional difference between `SUPER_ADMIN` and `OWNER`. The distinction above (technical/system administration vs. business ownership) is a recommended default, not a confirmed requirement. Until confirmed, the permission matrix below grants both roles identical business-data access and differs only on system-configuration permissions.

## 2. Permission Matrix

Legend: ✅ = allowed, ❌ = not allowed, **S** = scoped to assigned outlet(s) only, **G** = global regardless of outlet.

| Resource.Action | SUPER_ADMIN | OWNER | OUTLET_ADMIN | CASHIER | LAUNDRY_STAFF |
|---|---|---|---|---|---|
| user.create | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| user.read | ✅ G | ✅ G | ✅ S (own outlet staff) | ❌ | ❌ |
| user.update | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| role.manage / permission.manage | ✅ G | ❌ | ❌ | ❌ | ❌ |
| outlet.create | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| outlet.read | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| outlet.update | ✅ G | ✅ G | ✅ S (limited fields) | ❌ | ❌ |
| coverage_area.manage | ✅ G | ✅ G | ✅ S | ❌ | ❌ |
| service.manage (catalog) | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| pricing.manage | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| pricing.read | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| customer.create | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| customer.read | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| order.create | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| order.read | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| order.weigh | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| order.status.transition | ✅ G | ✅ G | ✅ S | ✅ S (limited, see `10-state-machines.md`) | ✅ S (limited) |
| order.transfer | ✅ G | ✅ G | ✅ S (as source outlet) | ❌ | ❌ |
| pickup.request | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| pickup.update | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| delivery.request | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| delivery.update | ✅ G | ✅ G | ✅ S | ✅ S | ✅ S |
| payment.create | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| payment.read | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| refund.request | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |
| refund.approve | ✅ G | ✅ G | ✅ S | ❌ | ❌ |
| refund.reject | ✅ G | ✅ G | ✅ S | ❌ | ❌ |
| report.read | ✅ G | ✅ G | ✅ S | ❌ | ❌ |
| order.override_payment_gate | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| tax_settings.manage | ✅ G | ✅ G | ❌ | ❌ | ❌ |
| tax_settings.read | ✅ G | ✅ G | ✅ S | ✅ S | ❌ |

`order.override_payment_gate` (**UQ-05 resolution**) authorizes the single explicit exception documented in `10-state-machines.md` §1.3: forcing `WEIGHING → WASHING` while `orders.payment_status <> 'PAID'`. It is deliberately restricted to `OWNER`/`SUPER_ADMIN` only — never `OUTLET_ADMIN` — because it bypasses the money-handling gate described in BR-02; every use is mandatory-`reason`, audited via `order_status_history.is_administrative_override` (§2.9 in `06-database-schema.md`), and does **not** waive the requirement that `payment_status = PAID` before `COMPLETED` (see state machine note). `tax_settings.manage` (**UQ-09 resolution**) authorizes writing a new `tax_rates` row (§2.14); also restricted to `OWNER`/`SUPER_ADMIN` since it changes a figure that affects every subsequent order's total.

Customers (guest or registered) hold an implicit, separate, non-staff permission set, enforced against a **customer-scoped bearer token** (issued by Identity Service on customer login, distinct token audience from staff tokens — **UQ-03 resolution**: `customer_accounts`/`customer_sessions`, see `06-database-schema.md` §1.9–1.10): `customer.self.read`, `customer.self.update`, `order.self.create`, `order.self.read`, `refund.self.request`, `payment.self.create` — always scoped to their own `customer_id`, never outlet-scoped, and **never** including `refund.approve`/`refund.reject` (BR-18) or `order.override_payment_gate`. Guest customers (no `customer_accounts` row) act without a bearer token: `order.self.create`/`order.self.read` for a guest order are instead authorized by a one-time order-access token or the `order_code` + phone number pair returned at creation — no permanent session is issued to a guest.

## 3. Enforcement Rule

Authorization is enforced **server-side only**, at the Gateway (coarse: is the route allowed for this role at all) and again at the service layer (fine: outlet-scope check against `user_outlets`, or resource-ownership check for customers). Frontend hiding of buttons/menus is a UX convenience and carries zero security weight — every service endpoint independently re-validates role + scope on every request, per Phase 0 spec §13.

## 4. Outlet Scope Resolution

1. Gateway forwards the authenticated principal's `user_id` and role claims (from the validated access token) to the downstream service via a trusted internal header/context (never trusts a client-supplied outlet id for authorization purposes).
2. The service loads `user_outlets` for that `user_id` (via Identity Service's exposed scope claim embedded in the token, refreshed at login/refresh — avoids a synchronous call on every request) to determine the allowed outlet set.
3. `OWNER`/`SUPER_ADMIN` bypass the `user_outlets` check entirely (global scope by role, per BR-21).
4. Any request whose target resource's `outlet_id` is not in the caller's allowed set is rejected with `403 OUTLET_OUT_OF_SCOPE`, even if the caller otherwise holds the permission.
