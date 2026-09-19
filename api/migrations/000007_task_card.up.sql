-- Ce que la carte du kanban affiche et que le schema ne portait pas.

-- 1. Les pieces jointes cessent d'appartenir au seul projet.
--
-- Plutot qu'une seconde table `task_files` qui aurait duplique la pile entiere
-- — requetes, magasin, endpoints de telechargement et de suppression — la
-- table existante accueille les deux proprietaires. Le CHECK garantit qu'il y
-- en a exactement un : ni piece jointe orpheline, ni piece jointe attachee des
-- deux cotes.
ALTER TABLE project_files RENAME TO attachments;
ALTER INDEX project_files_project_idx RENAME TO attachments_project_idx;

ALTER TABLE attachments
    ALTER COLUMN project_id DROP NOT NULL,
    ADD COLUMN task_id uuid REFERENCES tasks (id) ON DELETE CASCADE;

ALTER TABLE attachments
    ADD CONSTRAINT attachments_owner CHECK (num_nonnulls(project_id, task_id) = 1);

CREATE INDEX attachments_task_idx ON attachments (task_id);

-- 2. Compteurs de la carte.
--
-- Colonnes tenues par declencheur, comme les compteurs de taches du projet :
-- une carte de kanban ne doit declencher aucun COUNT au rendu, et le tableau
-- en affiche autant qu'il y a de taches.
ALTER TABLE tasks
    ADD COLUMN subtasks_total    integer NOT NULL DEFAULT 0,
    ADD COLUMN subtasks_done     integer NOT NULL DEFAULT 0,
    ADD COLUMN comments_count    integer NOT NULL DEFAULT 0,
    ADD COLUMN attachments_count integer NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION refresh_task_subtask_counts(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE tasks t
    SET subtasks_total = counts.total,
        subtasks_done  = counts.done
    FROM (
        SELECT count(*) AS total, count(*) FILTER (WHERE done) AS done
        FROM subtasks WHERE task_id = target
    ) AS counts
    WHERE t.id = target;
$$;

CREATE OR REPLACE FUNCTION subtasks_touch_task_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_task_subtask_counts(OLD.task_id);
        RETURN OLD;
    END IF;

    PERFORM refresh_task_subtask_counts(NEW.task_id);

    IF TG_OP = 'UPDATE' AND NEW.task_id <> OLD.task_id THEN
        PERFORM refresh_task_subtask_counts(OLD.task_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER subtasks_task_counts
AFTER INSERT OR UPDATE OF done, task_id OR DELETE ON subtasks
FOR EACH ROW EXECUTE FUNCTION subtasks_touch_task_counts();

CREATE OR REPLACE FUNCTION refresh_task_comment_count(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE tasks t
    SET comments_count = (
        SELECT count(*) FROM task_comments
        WHERE task_id = target AND deleted_at IS NULL
    )
    WHERE t.id = target;
$$;

CREATE OR REPLACE FUNCTION task_comments_touch_count()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_task_comment_count(OLD.task_id);
        RETURN OLD;
    END IF;

    PERFORM refresh_task_comment_count(NEW.task_id);
    RETURN NEW;
END;
$$;

CREATE TRIGGER task_comments_count
AFTER INSERT OR UPDATE OF deleted_at OR DELETE ON task_comments
FOR EACH ROW EXECUTE FUNCTION task_comments_touch_count();

CREATE OR REPLACE FUNCTION refresh_task_attachment_count(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE tasks t
    SET attachments_count = (SELECT count(*) FROM attachments WHERE task_id = target)
    WHERE t.id = target;
$$;

CREATE OR REPLACE FUNCTION attachments_touch_task_count()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.task_id IS NOT NULL THEN
            PERFORM refresh_task_attachment_count(OLD.task_id);
        END IF;
        RETURN OLD;
    END IF;

    IF NEW.task_id IS NOT NULL THEN
        PERFORM refresh_task_attachment_count(NEW.task_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER attachments_task_count
AFTER INSERT OR DELETE ON attachments
FOR EACH ROW EXECUTE FUNCTION attachments_touch_task_count();

-- Rattrapage des lignes deja en base : sans lui, les compteurs resteraient a
-- zero jusqu'a la prochaine ecriture sur chaque tache.
UPDATE tasks t SET
    subtasks_total    = (SELECT count(*) FROM subtasks WHERE task_id = t.id),
    subtasks_done     = (SELECT count(*) FROM subtasks WHERE task_id = t.id AND done),
    comments_count    = (SELECT count(*) FROM task_comments WHERE task_id = t.id AND deleted_at IS NULL),
    attachments_count = (SELECT count(*) FROM attachments WHERE task_id = t.id);
