-- Idempotency-Key ledger (docs/11-error-handling.md §4). Keys are meant to
-- expire after 24h — no cleanup job exists yet (TODO(phase-2+)); harmless
-- for correctness, just unbounded growth over time until one is added.
CREATE TABLE idempotency_keys (
    key             TEXT NOT NULL,
    endpoint        TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    response_status INT NOT NULL,
    response_body   BYTEA NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key, endpoint)
);
