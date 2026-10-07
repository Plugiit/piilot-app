DELETE FROM notifications WHERE kind = 'inbound_email_held';
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'task_created',
        'task_status_changed',
        'task_assigned',
        'task_unassigned',
        'task_commented',
        'task_due_changed',
        'project_created',
        'ticket_created',
        'ticket_assigned',
        'ticket_replied',
        'ticket_status_changed',
        'deliverable_validated',
        'deliverable_feedback',
        'update_available',
        'backup_stale'
    ));
DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE code = 'audit.read');
DELETE FROM permissions WHERE code = 'audit.read';
DROP TABLE IF EXISTS audit_log;
DROP FUNCTION IF EXISTS audit_log_guard();
DROP TABLE IF EXISTS ticket_reply_templates;
ALTER TABLE email_outbox DROP COLUMN IF EXISTS reply_to, DROP COLUMN IF EXISTS message_id, DROP COLUMN IF EXISTS in_reply_to;
ALTER TABLE ticket_messages DROP COLUMN IF EXISTS via_email, DROP COLUMN IF EXISTS sender_name, DROP COLUMN IF EXISTS sender_email;
ALTER TABLE tickets DROP COLUMN IF EXISTS requester_email, DROP COLUMN IF EXISTS requester_name;
DROP TABLE IF EXISTS inbound_emails;
DROP TABLE IF EXISTS inbound_mail_settings;
