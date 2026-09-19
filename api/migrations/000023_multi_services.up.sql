-- Un projet et une tache peuvent relever de plusieurs prestations.
--
-- La colonne unique ne tenait pas : une refonte, c'est du design ET du
-- developpement, et devoir choisir faisait perdre la moitie de l'information.
--
-- Deux tables de liaison plutot qu'un tableau de cles dans la ligne : une
-- table se joint, se compte et se contraint, la ou un array demanderait de
-- verifier a la main que chaque element designe un service vivant.
CREATE TABLE project_services (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES services (id) ON DELETE CASCADE,

    -- La cle primaire porte l'unicite : un service ne peut pas etre attache
    -- deux fois au meme projet, et l'ordre des colonnes sert la lecture la plus
    -- frequente — les services d'un projet.
    PRIMARY KEY (project_id, service_id)
);

-- Le sens inverse : « tous les projets de developpement web ».
CREATE INDEX project_services_service_idx ON project_services (service_id);

CREATE TABLE task_services (
    task_id    uuid NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    service_id uuid NOT NULL REFERENCES services (id) ON DELETE CASCADE,

    PRIMARY KEY (task_id, service_id)
);

CREATE INDEX task_services_service_idx ON task_services (service_id);

-- Reprise de ce qui etait deja etiquete : la colonne unique devient une ligne
-- de liaison. Aucune donnee ne se perd dans ce sens.
INSERT INTO project_services (project_id, service_id)
SELECT id, service_id FROM projects WHERE service_id IS NOT NULL AND deleted_at IS NULL;

INSERT INTO task_services (task_id, service_id)
SELECT id, service_id FROM tasks WHERE service_id IS NOT NULL AND deleted_at IS NULL;

DROP INDEX IF EXISTS projects_service_idx;
DROP INDEX IF EXISTS tasks_service_idx;

ALTER TABLE projects DROP COLUMN service_id;
ALTER TABLE tasks DROP COLUMN service_id;
