DROP TABLE IF EXISTS import_keys;
DROP TABLE IF EXISTS import_mappings;
DROP TABLE IF EXISTS import_rows;
DROP TABLE IF EXISTS imports;
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code IN ('data.import', 'data.export'));
DELETE FROM permissions WHERE code IN ('data.import', 'data.export');
