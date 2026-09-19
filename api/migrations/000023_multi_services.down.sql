-- Retour a un service unique.
--
-- La colonne ne peut en porter qu'un : les projets et les taches qui en
-- avaient plusieurs ne gardent que le premier par ordre alphabetique. C'est
-- une perte assumee, inherente au retour en arriere.
ALTER TABLE projects
    ADD COLUMN service_id uuid REFERENCES services (id) ON DELETE SET NULL;

ALTER TABLE tasks
    ADD COLUMN service_id uuid REFERENCES services (id) ON DELETE SET NULL;

UPDATE projects p SET service_id = (
    SELECT ps.service_id FROM project_services ps
    JOIN services s ON s.id = ps.service_id
    WHERE ps.project_id = p.id
    ORDER BY s.name, s.id
    LIMIT 1
);

UPDATE tasks t SET service_id = (
    SELECT ts.service_id FROM task_services ts
    JOIN services s ON s.id = ts.service_id
    WHERE ts.task_id = t.id
    ORDER BY s.name, s.id
    LIMIT 1
);

CREATE INDEX projects_service_idx
    ON projects (service_id) WHERE deleted_at IS NULL AND service_id IS NOT NULL;

CREATE INDEX tasks_service_idx
    ON tasks (service_id) WHERE deleted_at IS NULL AND service_id IS NOT NULL;

DROP TABLE IF EXISTS task_services;
DROP TABLE IF EXISTS project_services;
