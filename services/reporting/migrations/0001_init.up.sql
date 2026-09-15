-- Reporting Service schema. Source of truth: docs/06-database-schema.md §5.
-- Read-only projections, rebuildable from the event log — no outbox_events
-- table (Reporting publishes nothing); projection_checkpoints is its
-- idempotency ledger instead.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE rpt_daily_outlet_sales (
    sales_date        DATE NOT NULL,
    outlet_id         UUID NOT NULL,
    gross_amount      BIGINT NOT NULL DEFAULT 0,
    refunded_amount   BIGINT NOT NULL DEFAULT 0,
    order_count       INT NOT NULL DEFAULT 0,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (sales_date, outlet_id)
);

CREATE TABLE rpt_order_summary (
    order_id       UUID PRIMARY KEY,
    order_code     TEXT NOT NULL,
    outlet_id      UUID NOT NULL,
    customer_id    UUID NOT NULL,
    status         TEXT NOT NULL,
    total_amount   BIGINT,
    created_at     TIMESTAMPTZ NOT NULL,
    completed_at   TIMESTAMPTZ
);

CREATE TABLE rpt_service_performance (
    period_date       DATE NOT NULL,
    service_id        UUID NOT NULL,
    total_weight_kg   NUMERIC(12,3) NOT NULL DEFAULT 0,
    total_revenue     BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (period_date, service_id)
);

CREATE TABLE rpt_refund_summary (
    period_date     DATE NOT NULL,
    outlet_id       UUID NOT NULL,
    refund_count    INT NOT NULL DEFAULT 0,
    refund_amount   BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (period_date, outlet_id)
);

CREATE TABLE projection_checkpoints (
    projection_name     TEXT PRIMARY KEY,
    last_event_id       UUID,
    last_processed_at   TIMESTAMPTZ
);
