package handler

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// writeOutboxEvent inserts one outbox_events row inside tx — the entire
// point of the Transactional Outbox Pattern (docs/14-architecture-decisions.md
// ADR-009) is that this write commits atomically with the domain mutation
// that caused it, never separately.
func writeOutboxEvent(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`, aggregateType, aggregateID, eventType, body)
	return err
}

// markEventProcessed records an inbound event's id in Core's consumer
// dedup ledger (docs/06-database-schema.md §6: "an equivalent dedup table
// for Core consuming payment.*"), so a redelivered message is a safe no-op.
func markEventProcessed(ctx context.Context, tx pgx.Tx, eventID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`, eventID)
	return err
}
