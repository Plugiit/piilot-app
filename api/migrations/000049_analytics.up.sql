-- Module de statistiques : Piilot lit un service de mesure d'audience
-- (Umami pour commencer). Le service garde l'historique ; Piilot ne recopie
-- que les vues qu'il affiche, dans un cache que remplit une tache de fond —
-- aucun ecran n'appelle le service.

-- Reglages du module, une seule ligne. Mot de passe, cle d'API et jeton sont
-- chiffres.
CREATE TABLE analytics_settings (
    id                 integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled            boolean NOT NULL DEFAULT false,
    provider           text NOT NULL DEFAULT 'umami' CHECK (provider IN ('umami')),
    base_url           text NOT NULL DEFAULT '',
    -- login : instance auto-hebergee, identifiant et mot de passe ;
    -- api_key : Umami Cloud.
    auth_mode          text NOT NULL DEFAULT 'login' CHECK (auth_mode IN ('login', 'api_key')),
    username           text NOT NULL DEFAULT '',
    password_enc       bytea,
    api_key_enc        bytea,
    -- Le jeton de session, garde d'un demarrage a l'autre.
    token_enc          bytea,
    -- Version majeure du service : 0 inconnue, 2 ou 3. Deduite des reponses,
    -- ou choisie a la main quand version_pinned.
    api_version        smallint NOT NULL DEFAULT 0 CHECK (api_version IN (0, 2, 3)),
    version_pinned     boolean NOT NULL DEFAULT false,
    -- Duree de vie des vues courantes (7 j, 30 j, mois en cours).
    cache_minutes      integer NOT NULL DEFAULT 10 CHECK (cache_minutes BETWEEN 5 AND 15),
    last_check_at      timestamptz,
    last_check_error   text,
    -- Ce que la derniere verification a remarque sans echouer : une version
    -- qui change, par exemple.
    last_check_note    text,
    check_requested_at timestamptz,
    updated_at         timestamptz NOT NULL DEFAULT now()
);
INSERT INTO analytics_settings (id) VALUES (1);

-- Les sites du service. Un site supprime chez lui reste, marque absent : ses
-- liens et ses photos figees ne s'effacent pas avec lui.
CREATE TABLE analytics_websites (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider     text NOT NULL,
    external_id  text NOT NULL,
    name         text NOT NULL,
    domain       text NOT NULL DEFAULT '',
    present      boolean NOT NULL DEFAULT true,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

-- Les sites d'un projet. Le site de production est celui que voit le
-- client : un seul par projet.
CREATE TABLE project_analytics_websites (
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    website_id  uuid NOT NULL REFERENCES analytics_websites (id) ON DELETE CASCADE,
    environment text NOT NULL CHECK (environment IN ('prod', 'preprod')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, website_id)
);
CREATE UNIQUE INDEX project_analytics_websites_prod ON project_analytics_websites (project_id) WHERE environment = 'prod';
CREATE INDEX project_analytics_websites_website ON project_analytics_websites (website_id);

-- Le cache : une vue entiere (chiffres, courbe, classements) par site et par
-- periode. period_key : « 7d », « 30d », « month », « prev_month », « 12m »,
-- ou « custom:2026-01-01:2026-03-31 ».
CREATE TABLE analytics_cache (
    website_id   uuid NOT NULL REFERENCES analytics_websites (id) ON DELETE CASCADE,
    period_key   text NOT NULL,
    payload      jsonb NOT NULL,
    fetched_at   timestamptz NOT NULL DEFAULT now(),
    -- Derniere lecture par un ecran : une periode personnalisee que plus
    -- personne ne regarde se purge.
    last_read_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (website_id, period_key)
);

-- Les periodes personnalisees demandees par un ecran, servies par la tache.
-- Une demande identique en attente n'est pas dupliquee (index partiel) ; la
-- ligne servie reste un jour, pour compter les demandes d'un compte.
CREATE TABLE analytics_requests (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    website_id   uuid NOT NULL REFERENCES analytics_websites (id) ON DELETE CASCADE,
    period_key   text NOT NULL,
    requested_by uuid REFERENCES users (id) ON DELETE SET NULL,
    requested_at timestamptz NOT NULL DEFAULT now(),
    served_at    timestamptz,
    error        text
);
CREATE UNIQUE INDEX analytics_requests_pending ON analytics_requests (website_id, period_key) WHERE served_at IS NULL;
CREATE INDEX analytics_requests_by_user ON analytics_requests (requested_by, requested_at DESC);

-- Les visiteurs du moment, releves chaque minute, mais seulement pour un
-- site qu'un ecran regarde : last_read_at, touche a chaque lecture, le dit.
CREATE TABLE analytics_active (
    website_id   uuid PRIMARY KEY REFERENCES analytics_websites (id) ON DELETE CASCADE,
    visitors     integer,
    fetched_at   timestamptz,
    last_read_at timestamptz NOT NULL DEFAULT now()
);

-- Les chiffres d'un mois, figes le 1er du mois suivant : le compte rendu
-- mensuel les relit tels quels, meme si le service les recalcule ensuite.
CREATE TABLE analytics_snapshots (
    website_id uuid NOT NULL REFERENCES analytics_websites (id) ON DELETE CASCADE,
    month      date NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (website_id, month)
);
