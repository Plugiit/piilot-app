-- Jalons d'un projet.

-- name: ListProjectMilestones :many
-- Les jalons d'un projet, dans l'ordre de leurs echeances. Un projet en compte
-- une poignee ; la limite est la par principe.
SELECT *
FROM milestones
WHERE project_id = sqlc.arg('project_id')
ORDER BY due_on NULLS LAST, position, created_at
LIMIT 100;

-- name: ListMilestoneDeliverables :many
-- Les livrables rattaches aux jalons d'un projet, avec leur etat : la liste
-- des jalons les montre sous chacun, sans second appel.
SELECT
    d.id,
    d.title,
    d.milestone_id,
    coalesce(v.decision, 'brouillon')::text AS status
FROM deliverables d
LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
WHERE d.project_id = sqlc.arg('project_id')
  AND d.milestone_id IS NOT NULL
  AND d.deleted_at IS NULL
ORDER BY d.created_at
LIMIT 500;

-- name: GetMilestone :one
SELECT * FROM milestones WHERE id = $1;

-- name: CreateMilestone :one
INSERT INTO milestones (project_id, title, description, due_on, position, created_by)
VALUES (
    sqlc.arg('project_id'), sqlc.arg('title'), sqlc.arg('description'),
    sqlc.narg('due_on'),
    (SELECT coalesce(max(position), 0) + 1 FROM milestones WHERE project_id = sqlc.arg('project_id')),
    sqlc.narg('created_by')
)
RETURNING *;

-- name: UpdateMilestone :one
-- Un champ nul garde sa valeur ; l'echeance et l'etat ont chacun un drapeau,
-- parce que « effacer » et « ne pas toucher » s'y distinguent.
UPDATE milestones SET
    title        = coalesce(sqlc.narg('title'), title),
    description  = coalesce(sqlc.narg('description'), description),
    due_on       = CASE WHEN sqlc.arg('set_due')::boolean THEN sqlc.narg('due_on')::date ELSE due_on END,
    completed_at = CASE
        WHEN NOT sqlc.arg('set_completed')::boolean THEN completed_at
        WHEN sqlc.arg('completed')::boolean THEN coalesce(completed_at, now())
        ELSE NULL
    END,
    -- Une atteinte posee ou retiree a la main n'est plus celle des livrables :
    -- le declencheur ne la rouvrira pas.
    completed_by_deliverables = CASE
        WHEN sqlc.arg('set_completed')::boolean THEN false
        ELSE completed_by_deliverables
    END,
    updated_at   = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteMilestone :execrows
DELETE FROM milestones WHERE id = $1;

-- name: SetDeliverableMilestone :execrows
-- Rattache un livrable a un jalon de son propre projet, ou le detache. Le
-- projet est verifie dans la requete : un identifiant de jalon venu d'un autre
-- projet ne rattache rien.
UPDATE deliverables d SET
    milestone_id = sqlc.narg('milestone_id'),
    updated_at = now()
WHERE d.id = sqlc.arg('id')
  AND d.deleted_at IS NULL
  AND (
      sqlc.narg('milestone_id')::uuid IS NULL
      OR EXISTS (
          SELECT 1 FROM milestones m
          WHERE m.id = sqlc.narg('milestone_id')::uuid AND m.project_id = d.project_id
      )
  );

-- name: CreateMilestoneFromTemplate :exec
INSERT INTO milestones (project_id, title, due_on, position, created_by)
VALUES (sqlc.arg('project_id'), sqlc.arg('title'), sqlc.narg('due_on'), sqlc.arg('position'), sqlc.narg('created_by'));

-- name: ListPlanning :many
-- Le planning d'une periode : jalons, echeances de projets et taches de la
-- personne. Trois sources reunies en une requete, triees par date et bornees.
--
-- `mine` restreint jalons et projets a ceux ou la personne intervient ; les
-- taches sont toujours les siennes — celles de toute l'agence noieraient le
-- calendrier.
SELECT * FROM (
    SELECT
        'milestone'::text AS kind,
        m.id,
        m.title,
        m.due_on AS day,
        (m.completed_at IS NOT NULL)::boolean AS done,
        p.id   AS project_id,
        p.name AS project_name,
        m.deliverables_total,
        m.deliverables_validated
    FROM milestones m
    JOIN projects p ON p.id = m.project_id AND p.deleted_at IS NULL
    WHERE m.due_on BETWEEN sqlc.arg('from_day')::date AND sqlc.arg('to_day')::date
      AND (NOT sqlc.arg('mine')::boolean OR EXISTS (
          SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = sqlc.arg('user_id')
      ))

    UNION ALL

    SELECT
        'project_due'::text,
        p.id,
        p.name,
        p.due_on,
        (p.status IN ('livre', 'hebergement'))::boolean,
        p.id,
        p.name,
        0,
        0
    FROM projects p
    WHERE p.deleted_at IS NULL
      AND p.due_on BETWEEN sqlc.arg('from_day')::date AND sqlc.arg('to_day')::date
      AND (NOT sqlc.arg('mine')::boolean OR EXISTS (
          SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_id = sqlc.arg('user_id')
      ))

    UNION ALL

    SELECT
        'task_due'::text,
        t.id,
        t.title,
        t.due_on,
        (t.status = 'done')::boolean,
        p.id,
        p.name,
        0,
        0
    FROM task_assignees ta
    JOIN tasks t    ON t.id = ta.task_id AND t.deleted_at IS NULL
    JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
    WHERE ta.user_id = sqlc.arg('user_id')
      AND t.due_on BETWEEN sqlc.arg('from_day')::date AND sqlc.arg('to_day')::date
) planning
ORDER BY day, kind, title
LIMIT 600;
