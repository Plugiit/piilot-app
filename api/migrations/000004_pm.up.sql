-- Module PM : clients, projets, taches.
--
-- Schema pense pour ce que les ecrans affichent, pas pour decalquer un modele
-- existant. Trois consequences visibles ici :
--
--   1. Les agregats que les listes montrent (compte de taches, taches faites)
--      sont des colonnes de `projects`, tenues a jour par declencheur. Une
--      liste de projets ne fait donc aucun COUNT sur les taches.
--   2. Les libelles d'etat sont des CHECK et non des tables de reference :
--      quatre statuts figes que le front connait par coeur valent mieux qu'une
--      jointure sur chaque ligne pour lire « En cours ».
--   3. Tout ce qui se supprime depuis l'interface est efface logiquement
--      (deleted_at), parce qu'une tache supprimee par erreur doit pouvoir
--      revenir. Ce qui n'a pas de valeur propre — affectations, sous-taches —
--      part reellement, en cascade.

-- Clients de l'agence.
--
-- Le CRM viendra les enrichir (contacts multiples, interactions) ; cette table
-- porte le minimum sans lequel un projet n'a pas de sens : a qui il est vendu,
-- et qui repond au telephone.
CREATE TABLE clients (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          text        NOT NULL,
    contact_name  text        NOT NULL DEFAULT '',
    contact_email citext,
    contact_role  text        NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,

    CONSTRAINT clients_name_not_blank CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX clients_name_unique ON clients (lower(name)) WHERE deleted_at IS NULL;
-- Recherche floue sur le nom, pour le champ client du formulaire de projet.
CREATE INDEX clients_name_trgm_idx ON clients USING gin (name gin_trgm_ops);

CREATE TABLE projects (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id   uuid        NOT NULL REFERENCES clients (id) ON DELETE RESTRICT,
    name        text        NOT NULL,
    status      text        NOT NULL DEFAULT 'cadrage',
    -- Avancement declare par l'equipe, de 0 a 100. Distinct du rapport des
    -- taches faites : un projet peut etre a 80 % de son travail reel avec la
    -- moitie de ses taches ouvertes, et c'est le chef de projet qui tranche.
    progress    smallint    NOT NULL DEFAULT 0,
    -- Heures vendues, saisies au devis. Les heures consommees viendront des
    -- saisies de temps ; la colonne existe des maintenant pour que les ecrans
    -- aient leur place, et vaut 0 tant que le module temps n'est pas ecrit.
    hours_sold  numeric(7, 2) NOT NULL DEFAULT 0,
    hours_spent numeric(7, 2) NOT NULL DEFAULT 0,
    starts_on   date,
    due_on      date,
    -- Agregats des taches, maintenus par declencheur (voir plus bas).
    tasks_total integer     NOT NULL DEFAULT 0,
    tasks_done  integer     NOT NULL DEFAULT 0,
    created_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,

    CONSTRAINT projects_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT projects_status_check CHECK (status IN ('cadrage', 'production', 'attente', 'livre')),
    CONSTRAINT projects_progress_range CHECK (progress BETWEEN 0 AND 100),
    CONSTRAINT projects_hours_positive CHECK (hours_sold >= 0 AND hours_spent >= 0),
    -- Une echeance anterieure au demarrage est une faute de saisie, pas un
    -- etat metier : la base la refuse plutot que de laisser les ecrans
    -- afficher un retard de naissance.
    CONSTRAINT projects_dates_order CHECK (starts_on IS NULL OR due_on IS NULL OR due_on >= starts_on)
);

CREATE INDEX projects_client_idx ON projects (client_id) WHERE deleted_at IS NULL;
CREATE INDEX projects_status_idx ON projects (status) WHERE deleted_at IS NULL;
-- Tri par defaut de la liste : echeance croissante, les projets sans date en
-- dernier. L'index porte le meme ordre pour que la page 1 ne trie rien.
CREATE INDEX projects_due_idx ON projects (due_on NULLS LAST) WHERE deleted_at IS NULL;
CREATE INDEX projects_name_trgm_idx ON projects USING gin (name gin_trgm_ops);

-- Equipe affectee au projet.
--
-- Distincte des affectations de taches : quelqu'un peut suivre un projet sans
-- porter une tache, et l'ecran de projet montre l'equipe avant de montrer qui
-- fait quoi.
CREATE TABLE project_members (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX project_members_user_idx ON project_members (user_id);

CREATE TABLE tasks (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    title       text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    status      text        NOT NULL DEFAULT 'todo',
    -- Nature de la tache (Design, Integration, Contenu...). Texte libre et non
    -- table de reference : l'agence en invente au fil des projets, et une
    -- table imposerait un ecran d'administration pour une etiquette.
    tag         text        NOT NULL DEFAULT '',
    starts_on   date,
    due_on      date,
    -- Estimation en heures. Nulle tant que personne ne l'a donnee : zero
    -- voudrait dire « estimee a rien », ce qui n'est pas la meme chose.
    hours       numeric(6, 2),
    -- Note libre du panneau lateral.
    note        text        NOT NULL DEFAULT '',
    -- Rang dans sa colonne du tableau.
    position    integer     NOT NULL DEFAULT 0,
    -- Horodate le passage a « termine » : c'est ce qui permettra de dire
    -- combien de taches ont ete finies cette semaine sans rejouer le journal.
    completed_at timestamptz,
    created_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,

    CONSTRAINT tasks_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT tasks_status_check CHECK (status IN ('todo', 'progress', 'review', 'done')),
    CONSTRAINT tasks_hours_positive CHECK (hours IS NULL OR hours >= 0),
    CONSTRAINT tasks_dates_order CHECK (starts_on IS NULL OR due_on IS NULL OR due_on >= starts_on)
);

-- Index du tableau : toutes les taches vivantes d'un projet, rangees par
-- colonne puis par rang. C'est la seule lecture que fait l'ecran des taches.
CREATE INDEX tasks_board_idx ON tasks (project_id, status, position) WHERE deleted_at IS NULL;
CREATE INDEX tasks_due_idx ON tasks (due_on NULLS LAST) WHERE deleted_at IS NULL;
CREATE INDEX tasks_title_trgm_idx ON tasks USING gin (title gin_trgm_ops);

CREATE TABLE task_assignees (
    task_id    uuid NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, user_id)
);

-- Sens inverse : « mes taches », l'ecran qui viendra.
CREATE INDEX task_assignees_user_idx ON task_assignees (user_id);

CREATE TABLE subtasks (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id    uuid        NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    label      text        NOT NULL,
    done       boolean     NOT NULL DEFAULT false,
    position   integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT subtasks_label_not_blank CHECK (btrim(label) <> '')
);

CREATE INDEX subtasks_task_idx ON subtasks (task_id, position);

CREATE TABLE task_comments (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id    uuid        NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    author_id  uuid        REFERENCES users (id) ON DELETE SET NULL,
    body       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    CONSTRAINT task_comments_body_not_blank CHECK (btrim(body) <> '')
);

CREATE INDEX task_comments_task_idx ON task_comments (task_id, created_at DESC) WHERE deleted_at IS NULL;

-- Journal d'activite d'une tache.
--
-- Ecrit par l'applicatif et jamais modifie : c'est un historique, pas un etat.
-- `payload` porte ce que le libelle ne peut pas dire seul (l'ancien et le
-- nouveau statut, le nom de la sous-tache cochee) sous une forme qui n'oblige
-- pas a une colonne par type d'evenement.
CREATE TABLE task_activity (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id    uuid        NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    actor_id   uuid        REFERENCES users (id) ON DELETE SET NULL,
    kind       text        NOT NULL,
    payload    jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT task_activity_kind_check CHECK (kind IN (
        'created', 'status_changed', 'assigned', 'unassigned',
        'subtask_done', 'subtask_undone', 'commented', 'due_changed'
    ))
);

CREATE INDEX task_activity_task_idx ON task_activity (task_id, created_at DESC);

-- Compteurs de taches d'un projet.
--
-- Un declencheur plutot qu'un calcul dans l'applicatif : il n'existe qu'un
-- seul endroit ou la regle vit, et aucun chemin d'ecriture ne peut l'oublier —
-- pas meme une correction passee a la main en console. Le cout est une
-- ecriture de plus par mouvement de tache, sur une ligne deja verrouillee.
CREATE OR REPLACE FUNCTION refresh_project_task_counts(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE projects p
    SET tasks_total = counts.total,
        tasks_done = counts.done
    FROM (
        SELECT
            count(*) FILTER (WHERE deleted_at IS NULL)                          AS total,
            count(*) FILTER (WHERE deleted_at IS NULL AND status = 'done')      AS done
        FROM tasks
        WHERE project_id = target
    ) AS counts
    WHERE p.id = target;
$$;

CREATE OR REPLACE FUNCTION tasks_touch_project_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_project_task_counts(OLD.project_id);
        RETURN OLD;
    END IF;

    PERFORM refresh_project_task_counts(NEW.project_id);

    -- Une tache deplacee d'un projet a l'autre laisse un compteur a corriger
    -- derriere elle.
    IF TG_OP = 'UPDATE' AND NEW.project_id <> OLD.project_id THEN
        PERFORM refresh_project_task_counts(OLD.project_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_project_counts
AFTER INSERT OR UPDATE OF status, project_id, deleted_at OR DELETE ON tasks
FOR EACH ROW EXECUTE FUNCTION tasks_touch_project_counts();

-- `completed_at` suit le statut sans que chaque appelant ait a y penser.
CREATE OR REPLACE FUNCTION tasks_touch_completed_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status = 'done' AND (OLD.status IS DISTINCT FROM 'done') THEN
        NEW.completed_at := now();
    ELSIF NEW.status <> 'done' THEN
        NEW.completed_at := NULL;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_completed_at
BEFORE INSERT OR UPDATE OF status ON tasks
FOR EACH ROW EXECUTE FUNCTION tasks_touch_completed_at();
