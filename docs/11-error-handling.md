# 11 — Error Handling

## 1. Standard Error Envelope

Every non-2xx response uses this shape:

```json
{
  "error": {
    "code": "ORDER_ALREADY_PAID",
    "message": "This order has already been paid and cannot accept another payment.",
    "details": [
      { "field": "order_id", "issue": "order status is WASHING, expected WEIGHING" }
    ],
    "request_id": "req_9f8e...",
    "correlation_id": "corr_1a2b..."
  }
}
```

- `code`: stable, machine-readable, SCREAMING_SNAKE_CASE — safe for client `switch` logic. Never changes meaning once shipped (additive only).
- `message`: human-readable, safe to display, never leaks internals (no stack traces, no SQL).
- `details`: optional array for field-level validation errors.
- `request_id` / `correlation_id`: always present, ties the response to structured logs and traces (see `12-security-baseline.md` / observability).

## 2. HTTP Status Mapping

| HTTP status | Meaning | Example codes |
|---|---|---|
| 400 | Malformed request (bad JSON, missing required field at the transport level) | `MALFORMED_REQUEST` |
| 401 | Missing/invalid/expired credentials | `INVALID_CREDENTIALS`, `INVALID_TOKEN`, `TOKEN_EXPIRED` |
| 403 | Authenticated but not authorized | `FORBIDDEN_TRANSITION`, `OUTLET_OUT_OF_SCOPE` |
| 404 | Resource does not exist (or is outside caller's visibility, to avoid leaking existence) | `NOT_FOUND` |
| 409 | Conflict with current state | `ORDER_ALREADY_PAID`, `INVALID_TRANSITION`, `AMOUNT_MISMATCH`, `REFUND_NOT_ELIGIBLE`, `ITEM_LOCKED`, `INVALID_STATE` |
| 422 | Semantically invalid input (passes JSON parsing, fails business validation) | `VALIDATION_ERROR`, `NO_OUTLET_AVAILABLE` |
| 423 | Resource locked/inactive | `ACCOUNT_INACTIVE` |
| 429 | Rate limited | `RATE_LIMITED` |
| 500 | Unexpected server error (never exposes internals in `message`) | `INTERNAL_ERROR` |
| 503 | Downstream dependency unavailable (e.g., synchronous call to another service timed out) | `DEPENDENCY_UNAVAILABLE` |

## 3. Domain Error Catalog (non-exhaustive, extended per service as needed)

| Code | Service | Meaning |
|---|---|---|
| `ORDER_ALREADY_PAID` | Payment | Attempt to pay an order that already has a `PAID` payment (BR-06) |
| `AMOUNT_MISMATCH` | Payment | Submitted amount does not equal the order's authoritative total (BR-07) |
| `ITEM_LOCKED` | Core | Attempt to modify a weight/price/subtotal on a locked (post-payment) order item (BR-05) |
| `INVALID_TRANSITION` | Core | Requested order status transition is not in the allowed transition table |
| `FORBIDDEN_TRANSITION` | Core | Transition is valid in principle but not permitted for the caller's role |
| `REFUND_NOT_ELIGIBLE` | Payment | Refund requested against an order/payment that fails eligibility rules (BR-19) |
| `REQUIRES_REFUND_WORKFLOW` | Core | Attempt to directly cancel a paid order instead of going through refund |
| `NO_OUTLET_AVAILABLE` | Core | No outlet coverage matches and none was manually selected |
| `SAME_OUTLET` | Core | Transfer requested to the order's current outlet |
| `OUTLET_OUT_OF_SCOPE` | all | Caller's outlet scope does not include the target resource's outlet |

## 4. Idempotency Handling

- Clients supply `Idempotency-Key: <client-generated UUID>` on the endpoints listed in `07-api-contract.md` §12.
- Each service persists a small `idempotency_keys` table (per service, not shared — plumbing pattern lives in `shared/`, but the table itself is service-owned data): `(key, endpoint, request_hash, response_status, response_body, created_at)`, unique on `(key, endpoint)`.
- On a repeated request with the same key + endpoint:
  - If `request_hash` matches the original → return the **stored response** verbatim (same status code and body), without re-executing side effects.
  - If `request_hash` differs (same key reused for a different payload) → `409 IDEMPOTENCY_KEY_REUSE`.
- Keys expire after 24 hours (sufficient for client retry windows; not a permanent dedup ledger).
- This is the primary defense for `POST /payments` against double-charge on network retry, layered on top of the DB-level partial unique index (`06-database-schema.md` §3.1) as defense in depth.

## 5. Validation Error Detail Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "One or more fields are invalid.",
    "details": [
      { "field": "items[0].service_id", "issue": "must be a valid UUID referencing an active service" },
      { "field": "customer.phone", "issue": "required when customer.id is not provided" }
    ],
    "request_id": "req_...",
    "correlation_id": "corr_..."
  }
}
```

## 6. Cross-Service Failure Handling

- A synchronous read to another service (e.g., Payment → Core for order total) that times out or errors returns `503 DEPENDENCY_UNAVAILABLE` to the client rather than guessing/defaulting — financial correctness takes priority over availability for the payment path specifically.
- Retries for such reads use a short timeout (e.g., 2s) with 1 retry, then fail fast; circuit-breaking is a Phase 1 observability/resilience concern, flagged in `PHASE-0-REVIEW.md` as a Phase 1 prerequisite.
