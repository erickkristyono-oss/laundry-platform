// Package outbox implements the plumbing side of the Transactional Outbox
// Pattern (docs/06-database-schema.md §6, docs/14-architecture-decisions.md
// ADR-009): the envelope shape and a generic relay that polls a service's
// own outbox_events table and hands unpublished rows to a Publisher.
//
// It contains no domain knowledge of what any event *means* — only how an
// outbox row becomes a published message — so it belongs in shared/ per
// docs/03-system-architecture.md §4. Each service is still the one that
// writes its own outbox rows, inside its own domain transaction.
package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Envelope is the wire shape defined in docs/08-event-contract.md §2.
type Envelope struct {
	EventID       uuid.UUID       `json:"event_id"`
	EventType     string          `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Producer      string          `json:"producer"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	CorrelationID uuid.UUID       `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload"`
}

// Publisher sends an already-built Envelope to the broker (e.g. RabbitMQ,
// routing key = EventType). Implemented per service in internal/, not here,
// since exchange/queue topology is infrastructure config, not shared code.
type Publisher interface {
	Publish(ctx context.Context, env Envelope) error
}

type outboxRow struct {
	id            uuid.UUID
	aggregateType string
	aggregateID   uuid.UUID
	eventType     string
	eventVersion  int
	payload       json.RawMessage
	correlationID uuid.UUID
	createdAt     time.Time
}

// Relay polls `outbox_events` for unpublished rows and publishes them,
// marking each row published on success. It never deletes rows (audit
// trail) and retries indefinitely on publish failure, per
// docs/06-database-schema.md §6's retry/failure-handling contract.
type Relay struct {
	pool      *pgxpool.Pool
	publisher Publisher
	producer  string
	logger    *slog.Logger
	pollEvery time.Duration
	batchSize int
}

func NewRelay(pool *pgxpool.Pool, publisher Publisher, producer string, logger *slog.Logger) *Relay {
	return &Relay{
		pool:      pool,
		publisher: publisher,
		producer:  producer,
		logger:    logger,
		pollEvery: 2 * time.Second,
		batchSize: 100,
	}
}

// Run blocks, polling until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.tick(ctx); err != nil {
				r.logger.Error("outbox relay tick failed", slog.Any("error", err))
			}
		}
	}
}

// tick claims a batch of PENDING rows atomically (a single UPDATE...RETURNING
// using a SKIP LOCKED subquery, so multiple relay instances never claim the
// same row — see docs/06-database-schema.md §6 retry/failure-handling), then
// publishes each claimed row outside any open transaction (a network call
// must never hold a DB lock).
func (r *Relay) tick(ctx context.Context) error {
	rows, err := r.pool.Query(ctx, `
		UPDATE outbox_events
		SET status = 'PUBLISHING'
		WHERE id IN (
			SELECT id FROM outbox_events
			WHERE status = 'PENDING'
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, aggregate_type, aggregate_id, event_type, event_version, payload, created_at
	`, r.batchSize)
	if err != nil {
		return fmt.Errorf("outbox: claim pending: %w", err)
	}

	var claimed []outboxRow
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(&row.id, &row.aggregateType, &row.aggregateID, &row.eventType, &row.eventVersion, &row.payload, &row.createdAt); err != nil {
			rows.Close()
			return fmt.Errorf("outbox: scan: %w", err)
		}
		claimed = append(claimed, row)
	}
	rows.Close()

	for _, row := range claimed {
		env := Envelope{
			EventID:       row.id,
			EventType:     row.eventType,
			EventVersion:  row.eventVersion,
			OccurredAt:    row.createdAt,
			Producer:      r.producer,
			AggregateType: row.aggregateType,
			AggregateID:   row.aggregateID,
			CorrelationID: row.id, // fallback until correlation_id column carries the true business-operation id
			Payload:       row.payload,
		}

		if err := r.publisher.Publish(ctx, env); err != nil {
			r.logger.Warn("outbox publish failed, reverting to PENDING for retry",
				slog.String("event_id", row.id.String()),
				slog.String("event_type", row.eventType),
				slog.Any("error", err),
			)
			if _, revertErr := r.pool.Exec(ctx, `UPDATE outbox_events SET status = 'PENDING', retry_count = retry_count + 1 WHERE id = $1`, row.id); revertErr != nil {
				r.logger.Error("failed to revert claimed outbox row to PENDING", slog.String("event_id", row.id.String()), slog.Any("error", revertErr))
			}
			continue
		}

		if _, err := r.pool.Exec(ctx, `UPDATE outbox_events SET status = 'PUBLISHED', published_at = now() WHERE id = $1`, row.id); err != nil {
			r.logger.Error("failed to mark outbox row published after successful publish",
				slog.String("event_id", row.id.String()),
				slog.Any("error", err),
			)
		}
	}

	return nil
}
