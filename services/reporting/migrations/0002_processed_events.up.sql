-- Consumer-side idempotency ledger, mirroring Core's approach
-- (docs/06-database-schema.md §6: every consumer dedups on event_id).
-- projection_checkpoints exists in the schema for coarse replay tracking,
-- but a single last_event_id per projection cannot safely dedup arbitrary
-- redelivery from a durable queue — this table is what actually makes
-- reprocessing a redelivered message a no-op.
CREATE TABLE processed_events (
    event_id     UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
