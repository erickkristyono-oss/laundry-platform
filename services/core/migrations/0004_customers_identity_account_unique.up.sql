-- One identity_account_id must map to at most one customer profile
-- (UQ-03 resolution) — prevents a login from ever appearing to "own"
-- more than one customer record.
CREATE UNIQUE INDEX uq_customers_identity_account_id ON customers (identity_account_id) WHERE identity_account_id IS NOT NULL;
