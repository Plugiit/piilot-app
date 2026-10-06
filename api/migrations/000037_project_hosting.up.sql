-- Statut « hebergement » : un projet livre que l'agence continue d'heberger.
--
-- Il n'a plus d'echeance reelle ni d'avancement a suivre : cote production,
-- il compte comme un projet livre — ni actif, ni en retard, ni en alerte de
-- budget. Il reste pourtant vivant : le client y ouvre ses tickets, l'equipe
-- y saisit son temps de support.
ALTER TABLE projects DROP CONSTRAINT projects_status_check;
ALTER TABLE projects
    ADD CONSTRAINT projects_status_check
        CHECK (status IN ('cadrage', 'production', 'attente', 'livre', 'hebergement'));

-- « Projets actifs » d'un client : ceux qui sont encore en production.
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
          AND p.status NOT IN ('livre', 'hebergement')
    )
    WHERE c.id = target;
$$;
