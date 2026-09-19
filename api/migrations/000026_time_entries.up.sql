-- Temps passe.
--
-- `projects.hours_spent` existe depuis la migration 4 et n'a jamais rien recu :
-- elle ne servait qu'a trier la liste des projets, sur une valeur toujours
-- nulle. Cette table est ce qui la remplit.
CREATE TABLE time_entries (
    id      uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Qui a pointe. Chacun saisit le sien : l'identifiant vient de la session,
    -- jamais de la requete.
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,

    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    -- Nulle pour un echange client, une reunion, une relecture : tout le temps
    -- d'un projet ne se range pas sous une tache.
    --
    -- SET NULL et non CASCADE : supprimer une tache ne doit pas effacer des
    -- heures deja pointees. Elles restent au projet, qui les a bien consommees.
    task_id uuid REFERENCES tasks (id) ON DELETE SET NULL,

    -- La prestation sur laquelle l'heure compte. Une seule par ligne : on
    -- pointe une heure sur un service, pas sur deux a la fois — c'est ce qui
    -- rend les totaux additionnables.
    service_id uuid REFERENCES services (id) ON DELETE SET NULL,

    -- Le jour pointe, et non l'horodatage : on saisit le soir ou le lendemain,
    -- pas en temps reel.
    spent_on date NOT NULL,

    -- En minutes entieres. Vingt minutes valent 20 ; en heures il faudrait
    -- ecrire 0,33 et perdre un peu a chaque ligne. La conversion en heures se
    -- fait une seule fois, sur la somme.
    minutes integer NOT NULL,

    note text NOT NULL DEFAULT '',

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    -- Une saisie nulle ne dit rien, et une journee ne depasse pas vingt-quatre
    -- heures. La borne haute attrape la faute de frappe — 800 au lieu de 80.
    CONSTRAINT time_entries_minutes_check CHECK (minutes > 0 AND minutes <= 24 * 60)
);

-- L'ecran de saisie montre une journee, ou une semaine, pour une personne :
-- l'index porte les trois colonnes dans cet ordre de lecture.
CREATE INDEX time_entries_user_day_idx
    ON time_entries (user_id, spent_on DESC) WHERE deleted_at IS NULL;

-- Les rapports traversent : par projet, et par prestation.
CREATE INDEX time_entries_project_idx
    ON time_entries (project_id, spent_on DESC) WHERE deleted_at IS NULL;

CREATE INDEX time_entries_service_idx
    ON time_entries (service_id, spent_on DESC)
    WHERE deleted_at IS NULL AND service_id IS NOT NULL;

-- Heures consommees d'un projet.
--
-- Un declencheur plutot qu'un calcul dans l'applicatif, comme les compteurs de
-- taches : la regle vit a un seul endroit, et aucun chemin d'ecriture ne peut
-- l'oublier.
--
-- La division se fait sur la somme et non ligne a ligne : trois saisies de
-- vingt minutes font une heure pleine, la ou trois arrondis a 0,33 feraient
-- 0,99.
CREATE OR REPLACE FUNCTION refresh_project_hours_spent(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE projects p
    SET hours_spent = coalesce((
        SELECT round(sum(minutes)::numeric / 60, 2)
        FROM time_entries
        WHERE project_id = target AND deleted_at IS NULL
    ), 0)
    WHERE p.id = target;
$$;

CREATE OR REPLACE FUNCTION time_entries_touch_project_hours()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_project_hours_spent(OLD.project_id);
        RETURN OLD;
    END IF;

    PERFORM refresh_project_hours_spent(NEW.project_id);

    -- Une saisie deplacee d'un projet a l'autre laisse un total a corriger
    -- derriere elle.
    IF TG_OP = 'UPDATE' AND NEW.project_id <> OLD.project_id THEN
        PERFORM refresh_project_hours_spent(OLD.project_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER time_entries_project_hours
AFTER INSERT OR UPDATE OF minutes, project_id, deleted_at OR DELETE
ON time_entries
FOR EACH ROW EXECUTE FUNCTION time_entries_touch_project_hours();
