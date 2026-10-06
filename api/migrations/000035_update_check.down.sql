DELETE FROM notifications WHERE kind = 'update_available';
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
        'deliverable_feedback'
    ));
ALTER TABLE app_release_check
    DROP COLUMN IF EXISTS notified_version,
    DROP COLUMN IF EXISTS check_requested_at,
    DROP COLUMN IF EXISTS etag;
