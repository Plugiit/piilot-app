-- Activite de l'agence jour par jour, pour la carte « Activite par jour » du
-- tableau de bord.
--
-- Precalculee plutot que comptee au rendu : la carte couvre une annee entiere,
-- et la compter a chaque affichage reviendrait a parcourir toutes les taches,
-- tous les tickets et toutes les versions de livrables de l'annee. Une ligne
-- par jour, tenue par declencheur, se lit en une requete bornee a 366 lignes.
--
-- Trois natures, celles que la carte detaille au survol : une tache terminee,
-- un ticket ouvert, une version de livrable deposee. Ce sont des evenements,
-- pas un etat : supprimer un ticket n'efface pas le jour ou il a ete ouvert.
-- Seule exception, la tache rouverte, qui retire la terminaison qu'elle
-- annule — sinon une tache basculee dix fois compterait dix fois.
--
-- Le jour est celui du fuseau de la base, comme partout ailleurs.
CREATE TABLE daily_activity (
    day                    date    PRIMARY KEY,
    tasks_done             integer NOT NULL DEFAULT 0,
    tickets_opened         integer NOT NULL DEFAULT 0,
    deliverables_submitted integer NOT NULL DEFAULT 0
);

-- Ajoute (ou retire) des evenements a un jour, en creant sa ligne au besoin.
CREATE FUNCTION bump_daily_activity(target date, tasks integer, tickets integer, deliverables integer)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO daily_activity (day, tasks_done, tickets_opened, deliverables_submitted)
    VALUES (target, tasks, tickets, deliverables)
    ON CONFLICT (day) DO UPDATE SET
        tasks_done             = daily_activity.tasks_done + EXCLUDED.tasks_done,
        tickets_opened         = daily_activity.tickets_opened + EXCLUDED.tickets_opened,
        deliverables_submitted = daily_activity.deliverables_submitted + EXCLUDED.deliverables_submitted;
$$;

-- Une tache compte le jour de sa terminaison. `completed_at` est pose par le
-- declencheur BEFORE de la table : on lit donc la valeur finale ici, apres
-- coup. Le declencheur ne porte pas de liste de colonnes — `completed_at`
-- change sans figurer dans le SET de l'UPDATE, une liste ne le verrait pas.
CREATE FUNCTION tasks_touch_daily_activity() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.completed_at IS NOT NULL THEN
        PERFORM bump_daily_activity(OLD.completed_at::date, -1, 0, 0);
    END IF;

    IF NEW.completed_at IS NOT NULL THEN
        PERFORM bump_daily_activity(NEW.completed_at::date, 1, 0, 0);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_daily_activity_insert
    AFTER INSERT ON tasks
    FOR EACH ROW WHEN (NEW.completed_at IS NOT NULL)
    EXECUTE FUNCTION tasks_touch_daily_activity();

CREATE TRIGGER tasks_daily_activity_update
    AFTER UPDATE ON tasks
    FOR EACH ROW WHEN (OLD.completed_at IS DISTINCT FROM NEW.completed_at)
    EXECUTE FUNCTION tasks_touch_daily_activity();

CREATE FUNCTION tickets_touch_daily_activity() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM bump_daily_activity(NEW.created_at::date, 0, 1, 0);
    RETURN NEW;
END;
$$;

CREATE TRIGGER tickets_daily_activity
    AFTER INSERT ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_touch_daily_activity();

CREATE FUNCTION deliverable_versions_touch_daily_activity() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM bump_daily_activity(NEW.submitted_at::date, 0, 0, 1);
    RETURN NEW;
END;
$$;

CREATE TRIGGER deliverable_versions_daily_activity
    AFTER INSERT ON deliverable_versions
    FOR EACH ROW EXECUTE FUNCTION deliverable_versions_touch_daily_activity();

-- Reprise de l'existant : les declencheurs ne voient que l'avenir.
INSERT INTO daily_activity (day, tasks_done, tickets_opened, deliverables_submitted)
SELECT day, sum(tasks), sum(tickets), sum(deliverables)
FROM (
    SELECT completed_at::date AS day, count(*) AS tasks, 0 AS tickets, 0 AS deliverables
    FROM tasks WHERE completed_at IS NOT NULL GROUP BY 1
    UNION ALL
    SELECT created_at::date, 0, count(*), 0 FROM tickets GROUP BY 1
    UNION ALL
    SELECT submitted_at::date, 0, 0, count(*) FROM deliverable_versions GROUP BY 1
) AS events
GROUP BY day;
