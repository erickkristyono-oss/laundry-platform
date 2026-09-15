-- Core Service schema. Source of truth: docs/06-database-schema.md §2.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE outlets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    address    TEXT NOT NULL,
    phone      TEXT,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outlet_coverage_areas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    outlet_id   UUID NOT NULL REFERENCES outlets(id) ON DELETE CASCADE,
    area_label  TEXT NOT NULL,
    postal_code TEXT,
    priority    INT NOT NULL DEFAULT 100,
    UNIQUE (outlet_id, area_label)
);
CREATE INDEX idx_coverage_postal_code ON outlet_coverage_areas (postal_code);

CREATE TABLE customers (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_code        TEXT NOT NULL UNIQUE,
    name                 TEXT NOT NULL,
    phone                TEXT NOT NULL UNIQUE,
    email                TEXT,
    is_guest             BOOLEAN NOT NULL DEFAULT true,
    identity_account_id  UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_customers_phone ON customers (phone);

CREATE TABLE customer_addresses (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id   UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    label         TEXT NOT NULL,
    address_line  TEXT NOT NULL,
    postal_code   TEXT,
    is_default    BOOLEAN NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_customer_addresses_customer_id ON customer_addresses (customer_id);

CREATE TABLE services (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    unit       TEXT NOT NULL DEFAULT 'kg' CHECK (unit IN ('kg','pcs')),
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE service_prices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id      UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    outlet_id       UUID,
    price_per_unit  BIGINT NOT NULL CHECK (price_per_unit > 0),
    effective_from  TIMESTAMPTZ NOT NULL DEFAULT now(),
    effective_to    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_service_prices_lookup ON service_prices (service_id, outlet_id, effective_from);

-- UQ-09 resolution (docs/06-database-schema.md §2.14): versioned, admin-configurable PPN rate.
CREATE TABLE tax_rates (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rate_percentage  NUMERIC(5,2) NOT NULL CHECK (rate_percentage >= 0 AND rate_percentage <= 100),
    effective_from   TIMESTAMPTZ NOT NULL DEFAULT now(),
    effective_to     TIMESTAMPTZ,
    reason           TEXT NOT NULL,
    created_by       UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_tax_rates_active ON tax_rates (effective_to) WHERE effective_to IS NULL;
CREATE INDEX idx_tax_rates_effective ON tax_rates (effective_from, effective_to);

CREATE TABLE orders (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_code              TEXT NOT NULL UNIQUE,
    customer_id             UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    current_outlet_id       UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    source                  TEXT NOT NULL CHECK (source IN ('WEBSITE','POS','WHATSAPP')),
    fulfillment_type        TEXT NOT NULL DEFAULT 'WALK_IN' CHECK (fulfillment_type IN ('WALK_IN','PICKUP','DELIVERY','PICKUP_AND_DELIVERY')),
    status                  TEXT NOT NULL DEFAULT 'CREATED' CHECK (status IN (
        'CREATED','RECEIVED','WEIGHING','WASHING','DRYING','IRONING','PACKING',
        'READY','PICKED_UP','DELIVERED','COMPLETED','ON_HOLD','CANCELLED'
    )),
    payment_status          TEXT NOT NULL DEFAULT 'UNPAID' CHECK (payment_status IN ('UNPAID','PENDING','PAID','FAILED','REFUNDED')),
    estimated_total_amount  BIGINT,
    subtotal_amount         BIGINT,
    discount_amount         BIGINT NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    tax_amount              BIGINT NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    tax_rate_snapshot       NUMERIC(5,2) NOT NULL DEFAULT 0.00 CHECK (tax_rate_snapshot >= 0),
    pickup_fee              BIGINT NOT NULL DEFAULT 0 CHECK (pickup_fee >= 0),
    delivery_fee            BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee >= 0),
    total_amount            BIGINT CHECK (total_amount IS NULL OR total_amount >= 0),
    notes                   TEXT,
    is_archived             BOOLEAN NOT NULL DEFAULT false,
    archived_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ
);
CREATE INDEX idx_orders_customer_id ON orders (customer_id);
CREATE INDEX idx_orders_outlet_id ON orders (current_outlet_id);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_payment_status ON orders (payment_status);
CREATE INDEX idx_orders_created_at ON orders (created_at);

CREATE TABLE order_items (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id               UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    service_id             UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    service_name_snapshot  TEXT NOT NULL,
    unit                   TEXT NOT NULL,
    estimated_weight_kg    NUMERIC(10,3),
    actual_weight_kg       NUMERIC(10,3) CHECK (actual_weight_kg IS NULL OR actual_weight_kg >= 0),
    unit_price_snapshot    BIGINT NOT NULL,
    subtotal               BIGINT,
    is_locked              BOOLEAN NOT NULL DEFAULT false,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_items_order_id ON order_items (order_id);

-- Defense-in-depth enforcement of BR-05/BR-06 at the DB layer, in addition
-- to the application layer: once an item is locked (payment succeeded),
-- its financial/weight fields can never change again.
CREATE FUNCTION reject_locked_order_item_update() RETURNS TRIGGER AS $$
BEGIN
    IF OLD.is_locked THEN
        RAISE EXCEPTION 'order_items.%: cannot modify a locked line item (BR-05)', OLD.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_reject_locked_order_item_update
    BEFORE UPDATE OF actual_weight_kg, unit_price_snapshot, subtotal ON order_items
    FOR EACH ROW
    WHEN (OLD.is_locked = true)
    EXECUTE FUNCTION reject_locked_order_item_update();

CREATE TABLE order_status_history (
    id                            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id                      UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status                   TEXT,
    to_status                     TEXT NOT NULL,
    reason                        TEXT,
    performed_by                  UUID,
    is_administrative_override    BOOLEAN NOT NULL DEFAULT false,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_override_requires_reason CHECK (NOT is_administrative_override OR reason IS NOT NULL)
);
CREATE INDEX idx_order_status_history_order_id ON order_status_history (order_id);
CREATE INDEX idx_order_status_history_override ON order_status_history (order_id, created_at) WHERE is_administrative_override;

CREATE TABLE order_transfers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_outlet_id  UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    to_outlet_id    UUID NOT NULL REFERENCES outlets(id) ON DELETE RESTRICT,
    reason          TEXT NOT NULL CHECK (length(reason) > 0),
    performed_by    UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_order_transfers_order_id ON order_transfers (order_id);

CREATE TABLE pickup_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id      UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    address_id    UUID REFERENCES customer_addresses(id) ON DELETE SET NULL,
    status        TEXT NOT NULL DEFAULT 'REQUESTED' CHECK (status IN ('REQUESTED','SCHEDULED','COMPLETED','CANCELLED')),
    scheduled_at  TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE delivery_requests (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id      UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    address_id    UUID REFERENCES customer_addresses(id) ON DELETE SET NULL,
    status        TEXT NOT NULL DEFAULT 'REQUESTED' CHECK (status IN ('REQUESTED','SCHEDULED','COMPLETED','CANCELLED')),
    scheduled_at  TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

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
