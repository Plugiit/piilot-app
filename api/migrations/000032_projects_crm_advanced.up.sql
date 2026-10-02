-- Projets et CRM avances : jalons, modeles de projet, interactions client.

-- Jalons : les etapes datees d'un projet. Les livrables s'y rattachent ; un
-- jalon est tenu quand on le dit tenu, pas quand ses livrables le sont — une
-- mise en ligne peut etre atteinte avec un livrable encore en retours.
CREATE TABLE milestones (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    due_on      date,
    position    integer NOT NULL DEFAULT 0,

    -- Nul tant que le jalon n'est pas atteint. Une date plutot qu'un booleen :
    -- « atteint le 12 » se lit dans le planning.
    completed_at timestamptz,

    -- Compteurs de livrables tenus par declencheur : la liste des jalons et le
    -- planning les lisent sans rien compter.
    deliverables_total     integer NOT NULL DEFAULT 0,
    deliverables_validated integer NOT NULL DEFAULT 0,

    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT milestones_title_not_blank CHECK (btrim(title) <> '')
);

-- Un projet lit ses jalons dans l'ordre de leurs echeances.
CREATE INDEX milestones_project_idx ON milestones (project_id, due_on NULLS LAST, position);
-- Le planning lit une plage de dates, tous projets confondus.
CREATE INDEX milestones_due_idx ON milestones (due_on) WHERE due_on IS NOT NULL;

-- Un livrable appartient au plus a un jalon. SET NULL : supprimer un jalon ne
-- supprime pas le travail qui s'y rattachait.
ALTER TABLE deliverables
    ADD COLUMN milestone_id uuid REFERENCES milestones (id) ON DELETE SET NULL;

CREATE INDEX deliverables_milestone_idx ON deliverables (milestone_id)
    WHERE milestone_id IS NOT NULL AND deleted_at IS NULL;

CREATE OR REPLACE FUNCTION refresh_milestone_deliverable_counts(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE milestones m
    SET deliverables_total = c.total,
        deliverables_validated = c.validated
    FROM (
        SELECT
            count(*)::integer AS total,
            count(*) FILTER (WHERE v.decision = 'valide')::integer AS validated
        FROM deliverables d
        LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
        WHERE d.milestone_id = target AND d.deleted_at IS NULL
    ) c
    WHERE m.id = target;
$$;

CREATE OR REPLACE FUNCTION deliverables_touch_milestone_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') AND OLD.milestone_id IS NOT NULL THEN
        PERFORM refresh_milestone_deliverable_counts(OLD.milestone_id);
    END IF;

    IF TG_OP IN ('INSERT', 'UPDATE') AND NEW.milestone_id IS NOT NULL THEN
        PERFORM refresh_milestone_deliverable_counts(NEW.milestone_id);
    END IF;

    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER deliverables_milestone_counts
AFTER INSERT OR UPDATE OF milestone_id, current_version_id, deleted_at OR DELETE
ON deliverables
FOR EACH ROW EXECUTE FUNCTION deliverables_touch_milestone_counts();

-- Une decision rendue change le compte des valides sans toucher au livrable.
CREATE OR REPLACE FUNCTION deliverable_versions_touch_milestone_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    cible uuid;
BEGIN
    SELECT milestone_id INTO cible
    FROM deliverables
    WHERE id = CASE WHEN TG_OP = 'DELETE' THEN OLD.deliverable_id
                    ELSE NEW.deliverable_id END;

    IF cible IS NOT NULL THEN
        PERFORM refresh_milestone_deliverable_counts(cible);
    END IF;

    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER deliverable_versions_milestone_counts
AFTER INSERT OR UPDATE OF decision OR DELETE
ON deliverable_versions
FOR EACH ROW EXECUTE FUNCTION deliverable_versions_touch_milestone_counts();

-- Modeles de projet : de quoi demarrer un projet avec ses jalons, ses taches
-- et ses services deja poses. Les dates n'y sont pas absolues mais relatives
-- au debut du projet : un modele sert d'une annee a l'autre.
CREATE TABLE project_templates (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT project_templates_name_not_blank CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX project_templates_name_unique ON project_templates (lower(name));

CREATE TABLE project_template_services (
    template_id uuid NOT NULL REFERENCES project_templates (id) ON DELETE CASCADE,
    service_id  uuid NOT NULL REFERENCES services (id) ON DELETE CASCADE,
    PRIMARY KEY (template_id, service_id)
);

CREATE TABLE project_template_milestones (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id uuid NOT NULL REFERENCES project_templates (id) ON DELETE CASCADE,
    title       text NOT NULL,
    -- Jours apres le debut du projet.
    offset_days integer NOT NULL DEFAULT 0,
    position    integer NOT NULL DEFAULT 0,

    CONSTRAINT project_template_milestones_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT project_template_milestones_offset_check CHECK (offset_days BETWEEN 0 AND 3650)
);

CREATE INDEX project_template_milestones_idx ON project_template_milestones (template_id, position);

CREATE TABLE project_template_tasks (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id uuid NOT NULL REFERENCES project_templates (id) ON DELETE CASCADE,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    priority    text NOT NULL DEFAULT 'medium',
    -- Echeance en jours apres le debut du projet ; nulle, la tache n'en a pas.
    offset_days integer,
    position    integer NOT NULL DEFAULT 0,

    CONSTRAINT project_template_tasks_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT project_template_tasks_priority_check CHECK (priority IN ('low', 'medium', 'high')),
    CONSTRAINT project_template_tasks_offset_check CHECK (offset_days IS NULL OR offset_days BETWEEN 0 AND 3650)
);

CREATE INDEX project_template_tasks_idx ON project_template_tasks (template_id, position);

-- Interactions client : le journal de la relation. Notes, appels, rendez-vous
-- et e-mails sont saisis a la main ; les evenements (projet cree, livrable
-- valide, ticket ouvert) s'ecrivent seuls, dans la transaction du geste.
CREATE TABLE client_interactions (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id   uuid NOT NULL REFERENCES clients (id) ON DELETE CASCADE,
    kind        text NOT NULL,
    body        text NOT NULL DEFAULT '',
    occurred_at timestamptz NOT NULL DEFAULT now(),
    author_id   uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Projet concerne, s'il y en a un. SET NULL : l'historique de la relation
    -- survit a la suppression d'un projet.
    project_id  uuid REFERENCES projects (id) ON DELETE SET NULL,
    -- Ce que l'evenement dit, fige au moment du geste : titre du livrable,
    -- numero du ticket.
    payload     jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT client_interactions_kind_check CHECK (kind IN (
        'note', 'call', 'meeting', 'email',
        'project_created', 'deliverable_validated', 'ticket_opened'
    )),
    -- Une saisie a la main dit quelque chose ; un evenement peut se taire.
    CONSTRAINT client_interactions_body_check CHECK (
        kind NOT IN ('note', 'call', 'meeting', 'email') OR btrim(body) <> ''
    )
);

CREATE INDEX client_interactions_client_idx ON client_interactions (client_id, occurred_at DESC);
CREATE INDEX client_interactions_recent_idx ON client_interactions (occurred_at DESC);
