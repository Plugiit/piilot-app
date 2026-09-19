-- Livrables.
--
-- Le livrable est l'objet durable dont on parle — « la maquette d'accueil » —
-- et ses versions sont les allers-retours avec le client. C'est ce que le
-- portail donne a valider : `deliverables.validate` est la seule permission
-- d'ecriture accordee au role client (000002_rbac).
--
-- Cette permission est aussi portee par le role `team`, et c'est voulu : une
-- validation recue au telephone doit pouvoir etre enregistree par l'agence.
-- C'est le role du compte qui a tranche qui dit a quel titre.

CREATE TABLE deliverables (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',

    -- Derniere version soumise, ou nul tant qu'aucune ne l'a ete.
    --
    -- Pas de colonne « statut » a cote : l'etat d'un livrable est la decision
    -- de sa version courante, et deux colonnes pour un meme fait finissent par
    -- diverger. Le brouillon est l'absence de version, pas un statut de plus.
    -- La jointure qui le lit porte sur une cle primaire.
    current_version_id uuid,

    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    CONSTRAINT deliverables_title_check CHECK (title <> '')
);

CREATE TABLE deliverable_versions (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    deliverable_id uuid NOT NULL REFERENCES deliverables (id) ON DELETE CASCADE,
    -- Numero lisible au sein du livrable : « la v2 est partie ».
    numero         integer NOT NULL,

    -- Un fichier depose, ou un lien vers la ou le livrable vit deja :
    -- preproduction, maquette Figma. Une agence web livre autant de liens que
    -- de fichiers, et exporter un PDF d'une maquette pour la faire entrer dans
    -- un moule « fichier » ne rendrait service a personne.
    attachment_id uuid REFERENCES attachments (id) ON DELETE SET NULL,
    url           text NOT NULL DEFAULT '',

    submitted_at timestamptz NOT NULL DEFAULT now(),
    submitted_by uuid REFERENCES users (id) ON DELETE SET NULL,

    decision   text NOT NULL DEFAULT 'en_attente',
    decided_at timestamptz,
    decided_by uuid REFERENCES users (id) ON DELETE SET NULL,
    -- Ce que le client a repondu. Vide tant qu'il n'a pas tranche, et vide
    -- aussi pour une validation sans commentaire.
    feedback   text NOT NULL DEFAULT '',

    CONSTRAINT deliverable_versions_numero_unique
        UNIQUE (deliverable_id, numero),

    CONSTRAINT deliverable_versions_decision_check
        CHECK (decision IN ('en_attente', 'valide', 'retours')),

    -- L'un ou l'autre, jamais rien : une version qui ne designe rien n'est pas
    -- soumettable.
    CONSTRAINT deliverable_versions_support_check
        CHECK (attachment_id IS NOT NULL OR url <> ''),

    -- Une decision porte sa date, ou n'a pas ete prise. Sans cette contrainte,
    -- un « valide » sans date rendrait la trace inexploitable — c'est pourtant
    -- pour elle que le module existe.
    CONSTRAINT deliverable_versions_decided_check CHECK (
        (decision =  'en_attente' AND decided_at IS NULL)
     OR (decision <> 'en_attente' AND decided_at IS NOT NULL)
    )
);

-- Les deux tables se referencent : la cle se pose une fois les deux creees.
ALTER TABLE deliverables
    ADD CONSTRAINT deliverables_current_version_fkey
    FOREIGN KEY (current_version_id)
    REFERENCES deliverable_versions (id) ON DELETE SET NULL;

-- La fiche d'un projet liste ses livrables, du plus recent au plus ancien.
CREATE INDEX deliverables_project_idx
    ON deliverables (project_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- Le fil des versions d'un livrable, de la derniere a la premiere.
CREATE INDEX deliverable_versions_deliverable_idx
    ON deliverable_versions (deliverable_id, numero DESC);

-- « Qu'est-ce qui attend une reponse, et depuis quand » traverse les projets :
-- c'est l'ecran « Livrables » du module, et la raison d'etre de la file.
CREATE INDEX deliverable_versions_pending_idx
    ON deliverable_versions (submitted_at)
    WHERE decision = 'en_attente';

-- Compteur des livrables en attente d'un projet.
--
-- Un declencheur plutot qu'un calcul dans l'applicatif, comme les compteurs de
-- taches : la regle vit a un seul endroit, et aucun chemin d'ecriture ne peut
-- l'oublier.
ALTER TABLE projects
    ADD COLUMN deliverables_pending integer NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION refresh_project_deliverable_counts(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE projects p
    SET deliverables_pending = (
        SELECT count(*)
        FROM deliverables d
        JOIN deliverable_versions v ON v.id = d.current_version_id
        WHERE d.project_id = target
          AND d.deleted_at IS NULL
          AND v.decision = 'en_attente'
    )
    WHERE p.id = target;
$$;

CREATE OR REPLACE FUNCTION deliverables_touch_project_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_project_deliverable_counts(OLD.project_id);
        RETURN OLD;
    END IF;

    PERFORM refresh_project_deliverable_counts(NEW.project_id);

    -- Un livrable deplace d'un projet a l'autre laisse un compteur a corriger
    -- derriere lui.
    IF TG_OP = 'UPDATE' AND NEW.project_id <> OLD.project_id THEN
        PERFORM refresh_project_deliverable_counts(OLD.project_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER deliverables_project_counts
AFTER INSERT OR UPDATE OF project_id, current_version_id, deleted_at OR DELETE
ON deliverables
FOR EACH ROW EXECUTE FUNCTION deliverables_touch_project_counts();

-- Une decision rendue change le compteur sans toucher au livrable : le
-- declencheur precedent ne la verrait pas passer.
CREATE OR REPLACE FUNCTION deliverable_versions_touch_project_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    cible uuid;
BEGIN
    SELECT project_id INTO cible
    FROM deliverables
    WHERE id = CASE WHEN TG_OP = 'DELETE' THEN OLD.deliverable_id
                    ELSE NEW.deliverable_id END;

    -- Nul quand le livrable part dans la meme transaction : son propre
    -- declencheur a deja corrige le compteur.
    IF cible IS NOT NULL THEN
        PERFORM refresh_project_deliverable_counts(cible);
    END IF;

    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$;

CREATE TRIGGER deliverable_versions_project_counts
AFTER INSERT OR UPDATE OF decision OR DELETE
ON deliverable_versions
FOR EACH ROW EXECUTE FUNCTION deliverable_versions_touch_project_counts();
