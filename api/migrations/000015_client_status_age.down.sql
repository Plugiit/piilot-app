DROP TRIGGER IF EXISTS clients_status_changed ON clients;

DROP FUNCTION IF EXISTS touch_client_status_changed();

ALTER TABLE clients
    DROP COLUMN IF EXISTS status_changed_at;
