-- Ce que Piilot sait de la sante d'un site, sans rien executer dessus :
-- ses sauvegardes, qui se signalent elles-memes, et l'expiration de ses
-- domaines, relevee par une tache de fond.

-- Une source par projet. Le jeton n'est garde qu'en empreinte : il ne se
-- montre qu'a sa creation.
CREATE TABLE site_backup_sources (
    project_id           uuid PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    token_hash           bytea NOT NULL UNIQUE,
    -- Au-dela de ce delai sans sauvegarde reussie, les admins sont prevenus.
    expected_hours       integer NOT NULL DEFAULT 26 CHECK (expected_hours BETWEEN 1 AND 744),
    last_success_at      timestamptz,
    last_failure_at      timestamptz,
    -- Saisi a la main : une sauvegarde jamais restauree n'en est pas une.
    last_restore_test_on date,
    last_alert_at        timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE site_backup_runs (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    status      text NOT NULL CHECK (status IN ('success', 'failed')),
    size_bytes  bigint CHECK (size_bytes >= 0),
    message     text NOT NULL DEFAULT '',
    finished_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX site_backup_runs_project_idx ON site_backup_runs (project_id, finished_at DESC);

CREATE TABLE project_domains (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    domain           citext NOT NULL,
    expires_at       timestamptz,
    checked_at       timestamptz,
    last_success_at  timestamptz,
    last_error       text NOT NULL DEFAULT '',
    -- Le dernier palier annonce : chacun ne se dit qu'une fois.
    last_alert_days  integer,
    failure_alert_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, domain)
);
CREATE INDEX project_domains_checked_idx ON project_domains (checked_at NULLS FIRST);
CREATE INDEX project_domains_expires_idx ON project_domains (expires_at) WHERE expires_at IS NOT NULL;

-- Une mise a jour de securite se reconnait a son etiquette, que seul l'evenement
-- du webhook porte.
ALTER TABLE pull_requests ADD COLUMN security boolean NOT NULL DEFAULT false;
CREATE INDEX pull_requests_merged_idx ON pull_requests (project_id, merged_at) WHERE merged_at IS NOT NULL;

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
