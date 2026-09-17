-- Le service d'un projet et d'une tache.
--
-- Le referentiel ne servait a rien : rien ne le referencait. Une prestation
-- s'attache donc a ce qu'on vend — le projet — et a ce sur quoi on travaille —
-- la tache, qui pourra plus tard porter le temps passe par type de prestation.
--
-- Nullable des deux cotes : un projet interne ou une tache d'intendance ne
-- relevent d'aucune prestation, et forcer un choix ferait inventer un service
-- « Divers » qui ne dirait rien.
--
-- ON DELETE SET NULL plutot que RESTRICT : retirer une prestation du
-- referentiel ne doit pas bloquer sur dix ans de projets. En pratique la
-- suppression est douce, donc la cle ne se declenche pas — c'est la jointure
-- des lectures qui ecarte un service supprime, comme partout ailleurs.
ALTER TABLE projects
    ADD COLUMN service_id uuid REFERENCES services (id) ON DELETE SET NULL;

ALTER TABLE tasks
    ADD COLUMN service_id uuid REFERENCES services (id) ON DELETE SET NULL;

-- « Ce que l'agence a vendu en developpement web » traverse les projets : la
-- colonne se filtre, elle merite son index. Partiel comme les autres, et sans
-- les lignes nulles qui n'interessent aucune recherche par service.
CREATE INDEX projects_service_idx
    ON projects (service_id) WHERE deleted_at IS NULL AND service_id IS NOT NULL;

CREATE INDEX tasks_service_idx
    ON tasks (service_id) WHERE deleted_at IS NULL AND service_id IS NOT NULL;
