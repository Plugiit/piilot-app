DROP INDEX IF EXISTS projects_client_portal_idx;
ALTER TABLE attachments DROP COLUMN IF EXISTS shared_with_client;
