DELETE FROM notifications WHERE kind = 'site_update_failed';
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
        'report_attention',
        'site_backup_stale',
        'domain_expiring',
        'domain_check_failed'
    ));

DROP TABLE site_update_runs;
DROP TABLE site_update_targets;
DROP TABLE site_update_settings;
