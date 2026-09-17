-- La fiche d'un projet lit ses tickets du plus recemment mis a jour au plus
-- ancien, comme le tableau de l'assigne.
--
-- L'index ne portait que project_id : Postgres filtrait par l'index puis triait
-- le resultat. Il prend les deux colonnes dans l'ordre de lecture, comme
-- tickets_assignee_idx pose a la meme occasion — la premiere page se lit alors
-- sans tri.
DROP INDEX tickets_project_idx;

CREATE INDEX tickets_project_idx
    ON tickets (project_id, updated_at DESC)
    WHERE deleted_at IS NULL;
