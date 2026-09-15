-- Consumer-side idempotency ledger for events Core consumes from other
-- services (payment.*), per docs/06-database-schema.md §6's requirement
-- that every consumer dedup on event_id.
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
