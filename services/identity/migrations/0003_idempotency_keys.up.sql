-- Idempotency-Key ledger (docs/11-error-handling.md §4) — used by
-- POST /api/v1/customer-auth/register so a network retry never creates a
-- second account for the same submission.
CREATE TABLE idempotency_keys (
    key             TEXT NOT NULL,
    endpoint        TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    response_status INT NOT NULL,
    response_body   BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key, endpoint)
);
