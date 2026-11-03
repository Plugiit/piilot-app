DELETE FROM notifications WHERE kind IN ('site_backup_stale', 'domain_expiring', 'domain_check_failed');
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
        'backup_stale',
        'inbound_email_held',
        'report_attention'
    ));

DROP INDEX pull_requests_merged_idx;
ALTER TABLE pull_requests DROP COLUMN security;

DROP TABLE project_domains;
DROP TABLE site_backup_runs;
DROP TABLE site_backup_sources;
