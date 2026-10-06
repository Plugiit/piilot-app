-- Un projet heberge redevient un projet livre : c'est ce qu'il etait avant.
UPDATE projects SET status = 'livre' WHERE status = 'hebergement';

ALTER TABLE projects DROP CONSTRAINT projects_status_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_status_check
        CHECK (status IN ('cadrage', 'production', 'attente', 'livre'));

CREATE OR REPLACE FUNCTION refresh_client_projects_active(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE clients c
    SET projects_active = (
        SELECT count(*)
        FROM projects p
        WHERE p.client_id = target
          AND p.deleted_at IS NULL
          AND p.status <> 'livre'
    )
    WHERE c.id = target;
$$;
