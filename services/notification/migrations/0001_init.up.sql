-- Notification Service schema. Source of truth: docs/06-database-schema.md §4.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE notifications (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type                    TEXT NOT NULL,
    recipient_customer_id   UUID,
    recipient_user_id       UUID,
    source_event_id         UUID NOT NULL,
    payload                 JSONB NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_notifications_recipient CHECK (recipient_customer_id IS NOT NULL OR recipient_user_id IS NOT NULL)
);
CREATE UNIQUE INDEX uq_notifications_source_event_id ON notifications (source_event_id);

CREATE TABLE notification_deliveries (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id   UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel           TEXT NOT NULL CHECK (channel IN ('SMS','WHATSAPP','EMAIL','PUSH')),
    status            TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','SENT','FAILED')),
    attempt_count     INT NOT NULL DEFAULT 0,
    last_attempt_at   TIMESTAMPTZ,
    error_message     TEXT
);
CREATE INDEX idx_notification_deliveries_notification_id ON notification_deliveries (notification_id);

-- Outbox Table Standard (docs/06-database-schema.md §6). Present for
-- platform consistency; MVP has no downstream consumer of Notification's
-- own events (UQ-08, expected to stay empty).
CREATE TABLE outbox_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type TEXT NOT NULL,
    aggregate_id   UUID NOT NULL,
    event_type     TEXT NOT NULL,
    event_version  INT NOT NULL DEFAULT 1,
    payload        JSONB NOT NULL,
    status         TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PUBLISHING','PUBLISHED','FAILED')),
    retry_count    INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at   TIMESTAMPTZ
);
CREATE INDEX idx_outbox_status_created ON outbox_events (status, created_at);
