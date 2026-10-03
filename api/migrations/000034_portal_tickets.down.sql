DELETE FROM attachments WHERE ticket_id IS NOT NULL;
DROP INDEX IF EXISTS attachments_ticket_idx;
ALTER TABLE attachments DROP CONSTRAINT attachments_owner;
ALTER TABLE attachments
    ADD CONSTRAINT attachments_owner CHECK (num_nonnulls(project_id, task_id) = 1);
ALTER TABLE attachments DROP COLUMN IF EXISTS ticket_id;
DROP INDEX IF EXISTS tickets_client_visible_idx;
ALTER TABLE tickets DROP COLUMN IF EXISTS client_visible;
