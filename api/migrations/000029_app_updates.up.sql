-- Mise a jour de l'application depuis l'interface, reservee aux admins.
--
-- Trois acteurs, aucun appel entre eux : ils se parlent par ces tables.
--   - l'API, qui note la derniere version publiee (tache de fond, GitHub) et
--     enregistre les demandes des admins ;
--   - l'updater, un conteneur a part, seul a acceder a Docker, qui execute les
--     demandes et signale sa presence ;
--   - l'ecran, qui lit le tout.
-- L'API n'a ainsi jamais acces a Docker, et l'updater n'expose aucun port.

-- Derniere version publiee, telle que vue au dernier passage. Une seule ligne :
-- c'est un etat, pas un historique.
CREATE TABLE app_release_check (
    id           boolean     PRIMARY KEY DEFAULT true CHECK (id),
    version      text        NOT NULL DEFAULT '',
    name         text        NOT NULL DEFAULT '',
    url          text        NOT NULL DEFAULT '',
    published_at timestamptz,
    checked_at   timestamptz NOT NULL DEFAULT now(),
    -- Derniere erreur de verification, vide quand tout va bien.
    error        text        NOT NULL DEFAULT ''
);

-- Presence de l'updater, rafraichie toutes les trente secondes. Sans signe de
-- vie recent, le bouton n'est pas propose : une demande resterait en attente
-- sans personne pour l'executer.
CREATE TABLE app_updater (
    id        boolean     PRIMARY KEY DEFAULT true CHECK (id),
    version   text        NOT NULL DEFAULT '',
    -- Conteneur que l'updater mettra a jour, tel qu'il l'a trouve.
    target    text        NOT NULL DEFAULT '',
    -- Ce qui l'empeche de travailler (socket Docker absent, conteneur
    -- introuvable…), vide quand il est pret.
    error     text        NOT NULL DEFAULT '',
    last_seen timestamptz NOT NULL DEFAULT now()
);

-- Demandes de mise a jour. Historisees : qui a lance quoi, quand, et comment
-- ca s'est termine — une mise a jour qui casse doit pouvoir se retracer.
CREATE TABLE app_update_requests (
    id             uuid        PRIMARY KEY DEFAULT uuid_generate_v4(),
    requested_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    from_version   text        NOT NULL,
    target_version text        NOT NULL,
    status         text        NOT NULL DEFAULT 'pending',
    -- Etape en cours ou derniere etape franchie, pour le suivi a l'ecran.
    step           text        NOT NULL DEFAULT '',
    error          text        NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    started_at     timestamptz,
    finished_at    timestamptz,

    CONSTRAINT app_update_requests_status_check
        CHECK (status IN ('pending', 'running', 'done', 'failed'))
);

-- Une seule demande ouverte a la fois, arbitre par la base : deux admins qui
-- cliquent ensemble ne lancent pas deux mises a jour. Sert aussi d'index a
-- l'updater, qui ne lit que les demandes ouvertes.
CREATE UNIQUE INDEX app_update_requests_one_open
    ON app_update_requests ((true)) WHERE status IN ('pending', 'running');
CREATE INDEX app_update_requests_recent_idx ON app_update_requests (created_at DESC);

-- Permission dediee, accordee au seul role admin : mettre a jour l'application
-- coupe le service quelques instants pour tout le monde.
INSERT INTO permissions (code, label) VALUES
    ('system.update', 'Mettre a jour l''application');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code = 'system.update';
