DELETE FROM notifications WHERE kind = 'report_attention';
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
        'inbound_email_held'
    ));

DROP TABLE project_report_manual_review;

CREATE OR REPLACE FUNCTION reports_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'sent' THEN
        IF (to_jsonb(NEW) - 'validated_by') IS DISTINCT FROM (to_jsonb(OLD) - 'validated_by')
           OR (NEW.validated_by IS NOT NULL AND NEW.validated_by IS DISTINCT FROM OLD.validated_by) THEN
            RAISE EXCEPTION 'rapport envoye : il ne change plus' USING ERRCODE = 'check_violation';
        END IF;
    ELSIF OLD.status = 'validated' AND NEW.status = 'validated'
          AND (NEW.payload IS DISTINCT FROM OLD.payload OR NEW.editorial IS DISTINCT FROM OLD.editorial) THEN
        RAISE EXCEPTION 'rapport valide : repassez-le en brouillon pour le modifier' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE reports DROP COLUMN checked_at, DROP COLUMN undelivered, DROP COLUMN hold_reasons, DROP COLUMN held;

DROP INDEX email_outbox_report_idx;
ALTER TABLE email_outbox DROP COLUMN report_id;
