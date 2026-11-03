-- Fiabilite du rapport mensuel.
--
-- « Envoye » disait seulement « mis en file ». Le suivi de la remise distingue
-- desormais les deux : chaque e-mail connait son rapport, et le rapport garde
-- les adresses que la file a abandonnees, jusqu'a ce qu'on les relance.

ALTER TABLE email_outbox ADD COLUMN report_id uuid REFERENCES reports (id) ON DELETE SET NULL;
CREATE INDEX email_outbox_report_idx ON email_outbox (report_id) WHERE report_id IS NOT NULL;

ALTER TABLE reports
    -- Remis en relecture par un admin : l'envoi automatique ne le reprend pas.
    ADD COLUMN held boolean NOT NULL DEFAULT false,
    -- Ce qui retient l'envoi automatique, recalcule tant qu'il est brouillon.
    ADD COLUMN hold_reasons text[] NOT NULL DEFAULT '{}',
    -- Les adresses que la file d'envoi a abandonnees.
    ADD COLUMN undelivered text[] NOT NULL DEFAULT '{}',
    -- Derniere remise a jour d'un brouillon. generated_at ne bouge que si ses
    -- donnees changent : c'est lui qu'un admin valide en connaissance de cause.
    ADD COLUMN checked_at timestamptz;

-- Un rapport envoye reste fige, sauf le suivi de sa remise : ce n'est pas son
-- contenu.
CREATE OR REPLACE FUNCTION reports_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'sent' THEN
        -- Seul l'effacement d'un valideur supprime (compte efface) passe.
        IF (to_jsonb(NEW) - 'validated_by' - 'undelivered') IS DISTINCT FROM (to_jsonb(OLD) - 'validated_by' - 'undelivered')
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

-- Les projets dont le rapport ne part jamais tout seul.
CREATE TABLE project_report_manual_review (
    project_id uuid PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- L'alerte des rapports qui demandent un admin.
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
