-- Retablit l'index sur la seule colonne du projet.
DROP INDEX tickets_project_idx;

CREATE INDEX tickets_project_idx
    ON tickets (project_id)
    WHERE deleted_at IS NULL;
