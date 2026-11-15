-- 1.5 : droits RGPD -- export des donnees d'une personne (acces, portabilite)
-- et, au lot suivant, effacement avec delai.

INSERT INTO permissions (code, label) VALUES
    ('rgpd.manage', 'Exporter et effacer les donnees personnelles (RGPD)');
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code = 'rgpd.manage';

-- Une archive demandee, construite par une tache de fond, puis telechargee
-- avant son expiration. Le sujet n'a pas de cle etrangere : l'historique
-- survit a l'effacement du compte ou du client qu'il designe, et ne garde
-- que son libelle.
CREATE TABLE rgpd_exports (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject_type  text        NOT NULL,
    subject_id    uuid        NOT NULL,
    subject_label text        NOT NULL DEFAULT '',
    status        text        NOT NULL DEFAULT 'queued',
    -- Cle de l'archive dans le stockage ; vide quand elle n'existe pas ou plus.
    file_key      text        NOT NULL DEFAULT '',
    size_bytes    bigint      NOT NULL DEFAULT 0,
    requested_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    ready_at      timestamptz,
    expires_at    timestamptz,
    error         text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT rgpd_exports_subject_check CHECK (subject_type IN ('client', 'user')),
    CONSTRAINT rgpd_exports_status_check CHECK (status IN ('queued', 'running', 'ready', 'failed', 'expired')),
    CONSTRAINT rgpd_exports_size_check CHECK (size_bytes >= 0)
);

CREATE INDEX rgpd_exports_created_idx ON rgpd_exports (created_at DESC, id);
CREATE INDEX rgpd_exports_queued_idx ON rgpd_exports (created_at) WHERE status = 'queued';
CREATE INDEX rgpd_exports_expiry_idx ON rgpd_exports (expires_at) WHERE status = 'ready';

-- Effacements : la table est creee ici, le service arrive avec le lot suivant.
CREATE TABLE rgpd_erasures (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject_type   text        NOT NULL,
    subject_id     uuid        NOT NULL,
    subject_label  text        NOT NULL DEFAULT '',
    reason         text        NOT NULL DEFAULT 'request',
    status         text        NOT NULL DEFAULT 'scheduled',
    scheduled_for  timestamptz NOT NULL,
    requested_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    cancelled_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    cancelled_at   timestamptz,
    executed_at    timestamptz,
    summary        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    error          text        NOT NULL DEFAULT '',
    -- Etat « desactive » du compte avant la demande, pour le restaurer si
    -- l'effacement est annule.
    prev_disabled  jsonb,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT rgpd_erasures_subject_check CHECK (subject_type IN ('client', 'user')),
    CONSTRAINT rgpd_erasures_reason_check CHECK (reason IN ('request', 'inactivity')),
    CONSTRAINT rgpd_erasures_status_check CHECK (status IN ('scheduled', 'cancelled', 'running', 'done', 'failed'))
);

-- Une seule demande en cours par sujet.
CREATE UNIQUE INDEX rgpd_erasures_active_unique
    ON rgpd_erasures (subject_type, subject_id) WHERE status IN ('scheduled', 'running');
CREATE INDEX rgpd_erasures_due_idx ON rgpd_erasures (scheduled_for) WHERE status = 'scheduled';
CREATE INDEX rgpd_erasures_created_idx ON rgpd_erasures (created_at DESC, id);
