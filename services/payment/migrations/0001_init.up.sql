-- Payment Service schema. Source of truth: docs/06-database-schema.md §3.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE payments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_code  TEXT NOT NULL UNIQUE,
    order_id      UUID NOT NULL,
    customer_id   UUID NOT NULL,
    amount        BIGINT NOT NULL CHECK (amount > 0),
    method        TEXT NOT NULL CHECK (method IN ('CASH','QRIS','TRANSFER','CARD')),
    status        TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','PAID','FAILED','REFUNDED')),
    paid_at       TIMESTAMPTZ,
    is_archived   BOOLEAN NOT NULL DEFAULT false,
    archived_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- BR-06 (no partial/duplicate payment) enforced at the DB layer: only one
-- row per order can ever reach PAID.
CREATE UNIQUE INDEX uq_payments_order_id_status_paid ON payments (order_id) WHERE status = 'PAID';

CREATE TABLE payment_transactions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id         UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    transaction_type   TEXT NOT NULL CHECK (transaction_type IN ('CHARGE','CHARGE_FAILED','REFUND')),
    amount             BIGINT NOT NULL,
    status             TEXT NOT NULL CHECK (status IN ('SUCCESS','FAILED','PENDING')),
    gateway_response   JSONB,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_payment_transactions_payment_id ON payment_transactions (payment_id);

CREATE TABLE refunds (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    refund_code    TEXT NOT NULL UNIQUE,
    payment_id     UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    order_id       UUID NOT NULL,
    amount         BIGINT NOT NULL CHECK (amount > 0),
    status         TEXT NOT NULL DEFAULT 'REQUESTED' CHECK (status IN ('REQUESTED','APPROVED','PROCESSING','COMPLETED','FAILED','REJECTED')),
    reason         TEXT NOT NULL,
    requested_by   UUID NOT NULL,
    approved_by    UUID,
    processed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refunds_payment_id ON refunds (payment_id);

-- Outbox Table Standard (docs/06-database-schema.md §6).
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
