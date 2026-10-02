DELETE FROM permissions WHERE code = 'system.update';
DROP TABLE IF EXISTS app_update_requests;
DROP TABLE IF EXISTS app_updater;
DROP TABLE IF EXISTS app_release_check;
