-- Seeds roles and the permission matrix from docs/09-rbac.md §1-2.
-- Reference data, not business logic (allowed as a migration seed).

INSERT INTO roles (code, name) VALUES
    ('SUPER_ADMIN',   'Super Admin'),
    ('OWNER',         'Owner'),
    ('OUTLET_ADMIN',  'Outlet Admin'),
    ('CASHIER',       'Cashier'),
    ('LAUNDRY_STAFF', 'Laundry Staff');

INSERT INTO permissions (code, resource, action)
SELECT code, split_part(code, '.', 1), substring(code from position('.' in code) + 1)
FROM unnest(ARRAY[
    'user.create','user.read','user.update',
    'role.manage','permission.manage',
    'outlet.create','outlet.read','outlet.update',
    'coverage_area.manage',
    'service.manage',
    'pricing.manage','pricing.read',
    'customer.create','customer.read',
    'order.create','order.read','order.weigh','order.status.transition','order.transfer',
    'pickup.request','pickup.update',
    'delivery.request','delivery.update',
    'payment.create','payment.read',
    'refund.request','refund.approve','refund.reject',
    'report.read',
    'order.override_payment_gate',
    'tax_settings.manage','tax_settings.read'
]) AS code;

-- SUPER_ADMIN: every permission (docs/09-rbac.md §2).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.code = 'SUPER_ADMIN';

-- OWNER: every permission except role.manage / permission.manage.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'OWNER' AND p.code NOT IN ('role.manage', 'permission.manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code = ANY(ARRAY[
    'user.read','outlet.read','outlet.update','coverage_area.manage','pricing.read',
    'customer.create','customer.read','order.create','order.read','order.weigh',
    'order.status.transition','order.transfer','pickup.request','pickup.update',
    'delivery.request','delivery.update','payment.create','payment.read',
    'refund.request','refund.approve','refund.reject','report.read','tax_settings.read'
]) WHERE r.code = 'OUTLET_ADMIN';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code = ANY(ARRAY[
    'outlet.read','pricing.read','customer.create','customer.read','order.create',
    'order.read','order.weigh','order.status.transition','pickup.request','pickup.update',
    'delivery.request','delivery.update','payment.create','payment.read','refund.request',
    'tax_settings.read'
]) WHERE r.code = 'CASHIER';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.code = ANY(ARRAY[
    'outlet.read','pricing.read','customer.read','order.read','order.weigh',
    'order.status.transition','pickup.update','delivery.update'
]) WHERE r.code = 'LAUNDRY_STAFF';
