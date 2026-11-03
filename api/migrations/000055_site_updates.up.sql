-- Les mises a jour des sites, sur l'environnement choisi par l'agence.
--
-- Piilot n'execute rien : il demande a la CI du depot de lancer son workflow,
-- et attend son compte rendu. Une panne de Piilot ne peut donc pas toucher un
-- site, et un compte rendu qui n'arrive pas compte comme un echec.

-- Le jeton GitHub, chiffre. Une ligne : celui de l'agence.
CREATE TABLE site_update_settings (
    id               boolean PRIMARY KEY DEFAULT true CHECK (id),
    github_token_enc bytea NOT NULL,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- Pas de valeur par defaut pour l'environnement : c'est un choix explicite.
CREATE TABLE site_update_targets (
    project_id        uuid PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    environment       text NOT NULL CHECK (environment IN ('prod', 'preprod')),
    workflow          text NOT NULL,
    ref               text NOT NULL,
    -- Le depot confirme avec ce choix : s'il change sur le projet, plus rien ne
    -- part sans nouvelle confirmation.
    repo_url          text NOT NULL,
    -- Sans valeur : uniquement a la demande.
    every_days        integer CHECK (every_days BETWEEN 1 AND 90),
    enabled           boolean NOT NULL DEFAULT true,
    last_requested_at timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

-- Un lancement garde ce qui a ete demande : changer la cible ensuite ne reecrit
-- pas ce qui s'est passe.
CREATE TABLE site_update_runs (
    id                  uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id          uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    environment         text NOT NULL CHECK (environment IN ('prod', 'preprod')),
    workflow            text NOT NULL,
    ref                 text NOT NULL,
    repo_url            text NOT NULL,
    status              text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'dispatching', 'dispatched', 'success', 'failed', 'timed_out')),
    trigger             text NOT NULL CHECK (trigger IN ('manual', 'schedule')),
    requested_by        uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Valable pour ce seul lancement, efface a son compte rendu.
    callback_token_hash bytea,
    attempts            integer NOT NULL DEFAULT 0,
    next_attempt_at     timestamptz NOT NULL DEFAULT now(),
    message             text NOT NULL DEFAULT '',
    run_url             text NOT NULL DEFAULT '',
    requested_at        timestamptz NOT NULL DEFAULT now(),
    claimed_at          timestamptz,
    dispatched_at       timestamptz,
    finished_at         timestamptz
);
-- Un seul lancement actif par projet, quoi que fasse le code.
CREATE UNIQUE INDEX site_update_runs_active ON site_update_runs (project_id)
    WHERE status IN ('queued', 'dispatching', 'dispatched');
CREATE INDEX site_update_runs_project_idx ON site_update_runs (project_id, requested_at DESC);
CREATE INDEX site_update_runs_due_idx ON site_update_runs (next_attempt_at) WHERE status = 'queued';
CREATE UNIQUE INDEX site_update_runs_token_idx ON site_update_runs (callback_token_hash) WHERE callback_token_hash IS NOT NULL;

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
        'domain_check_failed',
        'site_update_failed'
    ));
