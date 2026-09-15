-- Seeds the initial active tax rate at 0% (UQ-09 resolution, docs/06-database-schema.md §2.14):
-- business turnover is currently below the Rp 500 juta/year PKP threshold.
-- created_by is the nil UUID to mark this as a system/migration seed rather
-- than an OWNER/SUPER_ADMIN action through the API.
INSERT INTO tax_rates (rate_percentage, reason, created_by)
VALUES (0.00, 'Di bawah ambang batas PKP Rp 500 juta/tahun', '00000000-0000-0000-0000-000000000000');
