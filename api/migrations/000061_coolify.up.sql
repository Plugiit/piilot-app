-- Module de deploiements : Piilot lit une instance Coolify (applications, puis
-- deploiements) pour que le rapport mensuel s'appuie sur ce qui a vraiment
-- ete deploye.

-- Reglages du module, une seule ligne. Le jeton d'API est chiffre.
CREATE TABLE coolify_settings (
    id                integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled           boolean NOT NULL DEFAULT false,
    base_url          text NOT NULL DEFAULT '',
    api_token_enc     bytea,
    poll_minutes      integer NOT NULL DEFAULT 60 CHECK (poll_minutes BETWEEN 15 AND 1440),
    last_poll_at      timestamptz,
    last_poll_error   text,
    poll_requested_at timestamptz,
    updated_at        timestamptz NOT NULL DEFAULT now()
);
INSERT INTO coolify_settings (id) VALUES (1);

-- Les applications de l'instance, telles que la derniere releve les a vues.
-- Une application qui disparait reste ici, marquee absente : les liens aux
-- projets et l'historique ne s'effacent pas avec elle.
CREATE TABLE coolify_applications (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    uuid             text NOT NULL UNIQUE,
    name             text NOT NULL,
    fqdn             text NOT NULL DEFAULT '',
    environment_name text NOT NULL DEFAULT '',
    project_name     text NOT NULL DEFAULT '',
    present          boolean NOT NULL DEFAULT true,
    last_seen_at     timestamptz NOT NULL DEFAULT now(),
    created_at       timestamptz NOT NULL DEFAULT now()
);
