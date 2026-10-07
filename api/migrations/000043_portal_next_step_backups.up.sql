-- 1.1 : interlocuteurs du portail et sauvegardes.

-- Lien de prise de rendez-vous d'un membre (Cal.com, Calendly, Google…),
-- montre au client sur le portail. Pas de creneaux dans Piilot : un lien.
ALTER TABLE users ADD COLUMN booking_url text NOT NULL DEFAULT '';

-- Journal des sauvegardes, ecrit par la commande `backup` : l'ecran des
-- parametres y lit la derniere reussie, la tache de fond y voit qu'elle date.
CREATE TABLE app_backups (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    -- running, done ou failed.
    status      text NOT NULL DEFAULT 'running',
    -- Ou la sauvegarde est posee : le dossier local, et le seau S3 le cas
    -- echeant.
    location    text NOT NULL DEFAULT '',
    size_bytes  bigint NOT NULL DEFAULT 0,
    error       text NOT NULL DEFAULT '',

    CONSTRAINT app_backups_status_check CHECK (status IN ('running', 'done', 'failed'))
);
CREATE INDEX app_backups_started_idx ON app_backups (started_at DESC);

-- Les admins sont prevenus quand la derniere sauvegarde reussie date.
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
