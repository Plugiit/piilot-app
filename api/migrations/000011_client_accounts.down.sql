DROP TRIGGER IF EXISTS users_client_counts ON users;
DROP TRIGGER IF EXISTS projects_client_counts ON projects;

DROP FUNCTION IF EXISTS users_touch_client_counts();
DROP FUNCTION IF EXISTS projects_touch_client_counts();
DROP FUNCTION IF EXISTS refresh_client_portal_users(uuid);
DROP FUNCTION IF EXISTS refresh_client_projects_active(uuid);

ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_counts_positive;

ALTER TABLE clients
    DROP COLUMN IF EXISTS portal_users,
    DROP COLUMN IF EXISTS projects_active;

DROP INDEX IF EXISTS users_client_idx;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_client_id_role_check;

ALTER TABLE users
    DROP COLUMN IF EXISTS client_id;
