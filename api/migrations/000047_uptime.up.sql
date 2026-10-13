-- Module de disponibilite : Piilot lit un service de surveillance (Uptime
-- Kuma pour commencer) et en garde l'historique, que le service n'expose pas.

-- Reglages du module, une seule ligne. La cle d'API est chiffree ; le secret
-- du webhook est en clair, il s'affiche pour etre colle dans Kuma.
CREATE TABLE uptime_settings (
    id                integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled           boolean NOT NULL DEFAULT false,
    provider          text NOT NULL DEFAULT 'kuma' CHECK (provider IN ('kuma')),
    base_url          text NOT NULL DEFAULT '',
    api_key_enc       bytea,
    poll_minutes      integer NOT NULL DEFAULT 2 CHECK (poll_minutes BETWEEN 1 AND 5),
    webhook_secret    text NOT NULL DEFAULT '',
    -- Dernier evenement recu par le webhook, pour l'ecran.
    webhook_last_at   timestamptz,
    last_poll_at      timestamptz,
    last_poll_error   text,
    poll_requested_at timestamptz,
    updated_at        timestamptz NOT NULL DEFAULT now()
);
INSERT INTO uptime_settings (id) VALUES (1);

-- Les sites surveilles, tels que la derniere releve les a vus. Un site qui
-- disparait du service reste ici, marque absent : son historique et ses
-- liens aux projets ne s'effacent pas avec lui.
CREATE TABLE uptime_monitors (
    id                uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider          text NOT NULL,
    -- « id:12 » ou « name:Site » : la cle du service.
    external_key      text NOT NULL,
    name              text NOT NULL,
    kind              text NOT NULL DEFAULT '',
    url               text NOT NULL DEFAULT '',
    hostname          text NOT NULL DEFAULT '',
    -- 0 hors ligne, 1 en ligne, 2 en attente, 3 maintenance.
    status            smallint NOT NULL CHECK (status BETWEEN 0 AND 3),
    response_ms       integer,
    cert_days         integer,
    cert_valid        boolean,
    present           boolean NOT NULL DEFAULT true,
    status_changed_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at      timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_key)
);

-- Un agregat par site et par jour, garde sans limite : c'est l'historique.
-- Une releve ratee compte dans checks_missing, jamais comme une panne : la
-- disponibilite se calcule sur up / (up + down).
CREATE TABLE uptime_daily (
    monitor_id     uuid NOT NULL REFERENCES uptime_monitors (id) ON DELETE CASCADE,
    day            date NOT NULL,
    checks_total   integer NOT NULL DEFAULT 0,
    checks_up      integer NOT NULL DEFAULT 0,
    checks_down    integer NOT NULL DEFAULT 0,
    checks_missing integer NOT NULL DEFAULT 0,
    ping_sum       bigint NOT NULL DEFAULT 0,
    ping_count     integer NOT NULL DEFAULT 0,
    ping_min       integer,
    ping_max       integer,
    PRIMARY KEY (monitor_id, day)
);

-- Les pannes : de la premiere alerte au retour. Un seul incident ouvert par
-- site, garanti par l'index partiel.
CREATE TABLE uptime_incidents (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    monitor_id       uuid NOT NULL REFERENCES uptime_monitors (id) ON DELETE CASCADE,
    started_at       timestamptz NOT NULL,
    ended_at         timestamptz,
    duration_seconds integer GENERATED ALWAYS AS (
        CASE WHEN ended_at IS NULL THEN NULL ELSE extract(epoch FROM ended_at - started_at)::integer END
    ) STORED,
    message          text NOT NULL DEFAULT '',
    -- webhook : pousse par le service ; poll : deduit de la releve.
    source           text NOT NULL CHECK (source IN ('webhook', 'poll')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);
CREATE UNIQUE INDEX uptime_incidents_open ON uptime_incidents (monitor_id) WHERE ended_at IS NULL;
CREATE INDEX uptime_incidents_recent ON uptime_incidents (started_at DESC);
CREATE INDEX uptime_incidents_monitor ON uptime_incidents (monitor_id, started_at DESC);

-- Les sites d'un projet : production ou preproduction. Facultatif.
CREATE TABLE project_uptime_monitors (
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    monitor_id  uuid NOT NULL REFERENCES uptime_monitors (id) ON DELETE CASCADE,
    environment text NOT NULL CHECK (environment IN ('prod', 'preprod')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, monitor_id)
);
CREATE INDEX project_uptime_monitors_monitor ON project_uptime_monitors (monitor_id);
