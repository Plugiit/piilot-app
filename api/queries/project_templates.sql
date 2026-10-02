-- Modeles de projet.

-- name: ListProjectTemplates :many
-- Les modeles, par ordre alphabetique, avec ce qu'ils contiennent. Les deux
-- comptes portent sur quelques lignes par modele, lues par index ; une agence
-- tient une poignee de modeles.
SELECT
    t.id,
    t.name,
    t.description,
    t.updated_at,
    (SELECT count(*) FROM project_template_milestones m WHERE m.template_id = t.id) AS milestones,
    (SELECT count(*) FROM project_template_tasks k WHERE k.template_id = t.id) AS tasks
FROM project_templates t
ORDER BY lower(t.name)
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountProjectTemplates :one
SELECT count(*) FROM project_templates;

-- name: GetProjectTemplate :one
SELECT * FROM project_templates WHERE id = $1;

-- name: ListTemplateServices :many
SELECT s.id, s.name, s.color
FROM project_template_services ts
JOIN services s ON s.id = ts.service_id
WHERE ts.template_id = $1
ORDER BY s.name;

-- name: ListTemplateMilestones :many
SELECT * FROM project_template_milestones
WHERE template_id = $1
ORDER BY position, offset_days
LIMIT 100;

-- name: ListTemplateTasks :many
SELECT * FROM project_template_tasks
WHERE template_id = $1
ORDER BY position
LIMIT 300;

-- name: CreateProjectTemplate :one
INSERT INTO project_templates (name, description, created_by)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateProjectTemplate :one
UPDATE project_templates SET
    name        = $2,
    description = $3,
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: DeleteProjectTemplate :execrows
DELETE FROM project_templates WHERE id = $1;

-- name: ClearTemplateContent :exec
-- Le contenu d'un modele s'enregistre d'un bloc : on efface puis on reecrit,
-- dans la meme transaction. Un modele n'a pas d'historique a garder.
WITH m AS (DELETE FROM project_template_milestones WHERE project_template_milestones.template_id = $1),
     s AS (DELETE FROM project_template_services WHERE project_template_services.template_id = $1)
DELETE FROM project_template_tasks WHERE project_template_tasks.template_id = $1;

-- name: AddTemplateService :exec
INSERT INTO project_template_services (template_id, service_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: AddTemplateMilestone :exec
INSERT INTO project_template_milestones (template_id, title, offset_days, position)
VALUES ($1, $2, $3, $4);

-- name: AddTemplateTask :exec
INSERT INTO project_template_tasks (template_id, title, description, priority, offset_days, position)
VALUES ($1, $2, $3, $4, $5, $6);
