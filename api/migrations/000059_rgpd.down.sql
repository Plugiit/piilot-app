DROP TABLE IF EXISTS rgpd_erasures;
DROP TABLE IF EXISTS rgpd_exports;
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code = 'rgpd.manage');
DELETE FROM permissions WHERE code = 'rgpd.manage';
