-- Identity Service schema. Source of truth: docs/06-database-schema.md §1.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          TEXT,
    phone          TEXT,
    full_name      TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    is_active      BOOLEAN NOT NULL DEFAULT true,
    last_login_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ,
    CONSTRAINT chk_users_identifier CHECK (email IS NOT NULL OR phone IS NOT NULL)
);
CREATE UNIQUE INDEX idx_users_email ON users (email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX idx_users_phone ON users (phone) WHERE phone IS NOT NULL;

CREATE TABLE roles (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_roles_code CHECK (code IN ('SUPER_ADMIN','OWNER','OUTLET_ADMIN','CASHIER','LAUNDRY_STAFF'))
);

CREATE TABLE permissions (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code     TEXT NOT NULL UNIQUE,
    resource TEXT NOT NULL,
    action   TEXT NOT NULL
);

CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE RESTRICT,
    PRIMARY KEY (role_id, permission_id)
);

-- outlet_id references core_db.outlets.id logically only — no cross-service FK
-- (docs/06-database-schema.md §0 / hard rule in docs/10-database-boundary via Master Prompt §10).
CREATE TABLE user_outlets (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    outlet_id  UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, outlet_id)
);
CREATE INDEX idx_user_outlets_outlet_id ON user_outlets (outlet_id);

CREATE TABLE sessions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash  TEXT NOT NULL UNIQUE,
    user_agent          TEXT,
    ip_address          INET,
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- customer_accounts / customer_sessions: UQ-03 resolution (docs/06-database-schema.md §1.9-1.10).
-- customer_id references core_db.customers.id logically only — no cross-service FK.
CREATE TABLE customer_accounts (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id        UUID NOT NULL UNIQUE,
    email              TEXT,
    phone              TEXT,
    password_hash      TEXT NOT NULL,
    email_verified_at  TIMESTAMPTZ,
    phone_verified_at  TIMESTAMPTZ,
    is_active          BOOLEAN NOT NULL DEFAULT true,
    last_login_at      TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ,
    CONSTRAINT chk_customer_accounts_identifier CHECK (email IS NOT NULL OR phone IS NOT NULL)
);
CREATE UNIQUE INDEX idx_customer_accounts_email ON customer_accounts (email) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX idx_customer_accounts_phone ON customer_accounts (phone) WHERE phone IS NOT NULL;

CREATE TABLE customer_sessions (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_account_id   UUID NOT NULL REFERENCES customer_accounts(id) ON DELETE CASCADE,
    refresh_token_hash    TEXT NOT NULL UNIQUE,
    user_agent            TEXT,
    ip_address            INET,
    expires_at            TIMESTAMPTZ NOT NULL,
    revoked_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_customer_sessions_account_id ON customer_sessions (customer_account_id);
CREATE INDEX idx_customer_sessions_expires_at ON customer_sessions (expires_at);

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
