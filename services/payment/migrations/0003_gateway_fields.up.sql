-- Online-gateway fields, additive to the CASH-only flow (docs/06-database-schema.md §3).
-- provider defaults to 'CASH' for existing/cash rows; a non-cash charge sets
-- it to whatever internal/gateway.Provider.Name() returned (e.g. 'DUMMY').
ALTER TABLE payments ADD COLUMN provider TEXT NOT NULL DEFAULT 'CASH';
ALTER TABLE payments ADD COLUMN provider_ref TEXT;
ALTER TABLE payments ADD COLUMN checkout_url TEXT;
ALTER TABLE payments ADD COLUMN qr_string TEXT;
