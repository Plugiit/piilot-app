-- 1.5 : import et export de donnees en CSV.

INSERT INTO permissions (code, label) VALUES
    ('data.import', 'Importer des donnees depuis un fichier CSV'),
    ('data.export', 'Exporter des donnees en CSV');
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code IN ('data.import', 'data.export');

-- Un fichier depose, puis son traitement par une tache de fond : aucune
-- requete HTTP n'attend la fin d'un import.
CREATE TABLE imports (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity      text        NOT NULL,
    status      text        NOT NULL DEFAULT 'uploaded',
    -- Simulation : meme code, transaction annulee a la fin.
    dry_run     boolean     NOT NULL DEFAULT false,
    -- Cle du fichier dans le stockage ; vide une fois le fichier supprime.
    file_key    text        NOT NULL DEFAULT '',
    file_name   text        NOT NULL DEFAULT '',
    size_bytes  bigint      NOT NULL DEFAULT 0,
    delimiter   text        NOT NULL DEFAULT ';',
    encoding    text        NOT NULL DEFAULT 'utf-8',
    headers     text[]      NOT NULL DEFAULT '{}',
    row_count   integer     NOT NULL DEFAULT 0,
    -- Colonne du fichier -> champ de Piilot.
    mapping     jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_n   integer     NOT NULL DEFAULT 0,
    skipped_n   integer     NOT NULL DEFAULT 0,
    error_n     integer     NOT NULL DEFAULT 0,
    error       text        NOT NULL DEFAULT '',
    created_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    started_at  timestamptz,
    finished_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT imports_entity_check CHECK (entity IN ('clients', 'contacts', 'projects', 'time_entries')),
    CONSTRAINT imports_status_check CHECK (status IN ('uploaded', 'queued', 'running', 'done', 'failed', 'cancelled')),
    CONSTRAINT imports_counts_check CHECK (row_count >= 0 AND created_n >= 0 AND skipped_n >= 0 AND error_n >= 0)
);

CREATE INDEX imports_created_idx ON imports (created_at DESC);
-- Ce que la tache de fond reclame : quelques lignes au plus.
CREATE INDEX imports_queued_idx ON imports (created_at) WHERE status = 'queued';

-- Le rapport, ligne par ligne. `reason` est un code stable, le texte est
-- ecrit par l'ecran.
CREATE TABLE import_rows (
    import_id uuid    NOT NULL REFERENCES imports (id) ON DELETE CASCADE,
    line      integer NOT NULL,
    outcome   text    NOT NULL,
    reason    text    NOT NULL DEFAULT '',
    label     text    NOT NULL DEFAULT '',
    entity_id uuid,

    PRIMARY KEY (import_id, line),
    CONSTRAINT import_rows_outcome_check CHECK (outcome IN ('created', 'skipped', 'error'))
);

CREATE INDEX import_rows_outcome_idx ON import_rows (import_id, outcome, line);

-- Le dernier mapping utilise pour chaque entite, propose au depot suivant.
CREATE TABLE import_mappings (
    entity     text PRIMARY KEY,
    mapping    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    updated_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT import_mappings_entity_check CHECK (entity IN ('clients', 'contacts', 'projects', 'time_entries'))
);

-- Ce qui a deja ete importe, pour qu'un fichier rejoue ne cree rien deux
-- fois. Des empreintes seulement, jamais les donnees.
CREATE TABLE import_keys (
    entity     text        NOT NULL,
    source_key text        NOT NULL,
    entity_id  uuid        NOT NULL,
    import_id  uuid        REFERENCES imports (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (entity, source_key),
    CONSTRAINT import_keys_entity_check CHECK (entity IN ('clients', 'contacts', 'projects', 'time_entries'))
);

CREATE INDEX import_keys_entity_id_idx ON import_keys (entity_id);
CREATE INDEX import_keys_import_idx ON import_keys (import_id) WHERE import_id IS NOT NULL;
