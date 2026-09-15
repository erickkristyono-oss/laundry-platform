-- Idempotency-Key ledger (docs/11-error-handling.md §4) — the primary
-- defense for POST /payments against double-charge on network retry,
-- layered on top of the DB-level partial unique index on payments.order_id.
CREATE TABLE idempotency_keys (
    key             TEXT NOT NULL,
    endpoint        TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    response_status INT NOT NULL,
    response_body   BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key, endpoint)
);
