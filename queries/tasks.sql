-- name: ListTasksOfProject :many
-- Tableau des taches d'un projet.
--
-- Bornee comme toutes les listes. Un tableau n'a pas de pagination visible —
-- on ne tourne pas la page d'un kanban — mais la borne existe quand meme :
-- c'est elle qui empeche un projet devenu fourre-tout de ramener dix mille
-- lignes. Le handler renvoie le total a cote, pour que l'ecran puisse dire
-- qu'il n'affiche pas tout.
SELECT * FROM tasks
WHERE project_id = sqlc.arg('project_id') AND deleted_at IS NULL
ORDER BY status, position, created_at
LIMIT sqlc.arg('page_size');

-- name: ListTasks :many
-- Taches de toute l'agence, pour l'ecran « Taches » du module.
--
-- Le nom du projet est joint ici : une tache sortie de sa fiche ne dit plus
-- d'ou elle vient, et l'aller chercher ensuite ferait une requete par ligne.
--
-- Meme borne que le tableau d'un projet, et pour la meme raison : l'ecran a
-- une vue kanban, et on ne tourne pas la page d'un kanban. Le total part a
-- cote pour que la vue puisse dire qu'elle n'affiche pas tout.
--
-- La jointure sur les projets vivants fait le reste du filtrage : les taches
-- d'un projet supprime ne doivent pas reapparaitre dans une liste globale.
SELECT
    t.*,
    p.name AS project_name
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (sqlc.narg('priority')::text IS NULL OR t.priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR t.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR t.title ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY t.status, t.position, t.created_at, t.id
LIMIT sqlc.arg('page_size');

-- name: CountTasks :one
SELECT count(*)
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (sqlc.narg('priority')::text IS NULL OR t.priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR t.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR t.title ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: CountTasksOfProject :one
SELECT count(*) FROM tasks
WHERE project_id = $1 AND deleted_at IS NULL;

-- name: ListAssigneesOfTasks :many
-- Affectations de plusieurs taches en une requete : meme parade au N+1 que
-- pour les equipes de projets.
SELECT
    ta.task_id,
    u.id,
    u.firstname,
    u.lastname,
    u.avatar_url
FROM task_assignees ta
JOIN users u ON u.id = ta.user_id
WHERE ta.task_id = ANY(sqlc.arg('task_ids')::uuid[])
  AND u.deleted_at IS NULL
ORDER BY ta.task_id, u.firstname, u.lastname;

-- name: GetTask :one
-- Tache et son contexte de projet : le tiroir affiche le nom du projet, et
-- l'aller chercher a part ferait une requete de plus pour un seul mot.
SELECT
    t.*,
    p.name AS project_name,
    c.name AS client_name
FROM tasks t
JOIN projects p ON p.id = t.project_id
JOIN clients c ON c.id = p.client_id
WHERE t.id = $1 AND t.deleted_at IS NULL;

-- name: CreateTask :one
INSERT INTO tasks (project_id, title, description, status, tag, priority, starts_on, due_on, hours, note, position, created_by)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    -- Rang calcule dans la requete d'insertion : un SELECT max() suivi d'un
    -- INSERT laisserait deux creations simultanees se donner le meme rang.
    (SELECT coalesce(max(position), 0) + 1 FROM tasks
     WHERE project_id = $1 AND status = $4 AND deleted_at IS NULL),
    $11
)
RETURNING *;

-- name: UpdateTask :one
UPDATE tasks SET
    title       = COALESCE(sqlc.narg('title')::text, title),
    description = COALESCE(sqlc.narg('description')::text, description),
    tag         = COALESCE(sqlc.narg('tag')::text, tag),
    priority    = COALESCE(sqlc.narg('priority')::text, priority),
    note        = COALESCE(sqlc.narg('note')::text, note),
    hours       = CASE WHEN sqlc.arg('clear_hours')::boolean THEN NULL
                       ELSE COALESCE(sqlc.narg('hours')::numeric, hours) END,
    starts_on   = CASE WHEN sqlc.arg('clear_starts_on')::boolean THEN NULL
                       ELSE COALESCE(sqlc.narg('starts_on')::date, starts_on) END,
    due_on      = CASE WHEN sqlc.arg('clear_due_on')::boolean THEN NULL
                       ELSE COALESCE(sqlc.narg('due_on')::date, due_on) END,
    updated_at  = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: MoveTask :one
-- Deplacement dans le tableau : la colonne, et le rang dans cette colonne.
-- Separe de UpdateTask parce que c'est le seul mouvement qui journalise un
-- changement de statut, et que le tableau l'appelle a chaque glissement.
UPDATE tasks AS t SET
    status     = sqlc.arg('status')::text,
    -- La table cible est aliasee : sans cela, le `tasks` de la sous-requete
    -- masque celui de l'UPDATE et la colonne devient ambigue.
    position   = COALESCE(
        sqlc.narg('position')::integer,
        (SELECT coalesce(max(sibling.position), 0) + 1 FROM tasks AS sibling
         WHERE sibling.project_id = t.project_id
           AND sibling.status = sqlc.arg('status')::text
           AND sibling.deleted_at IS NULL)
    ),
    updated_at = now()
WHERE t.id = sqlc.arg('id') AND t.deleted_at IS NULL
RETURNING t.*;

-- name: SoftDeleteTask :exec
UPDATE tasks SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: AssignTask :exec
INSERT INTO task_assignees (task_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnassignTask :exec
DELETE FROM task_assignees WHERE task_id = $1 AND user_id = $2;

-- name: ListSubtasks :many
SELECT * FROM subtasks
WHERE task_id = $1
ORDER BY position, created_at;

-- name: CreateSubtask :one
INSERT INTO subtasks (task_id, label, position)
VALUES (
    $1, $2,
    (SELECT coalesce(max(position), 0) + 1 FROM subtasks WHERE task_id = $1)
)
RETURNING *;

-- name: UpdateSubtask :one
UPDATE subtasks SET
    label      = COALESCE(sqlc.narg('label')::text, label),
    done       = COALESCE(sqlc.narg('done')::boolean, done),
    position   = COALESCE(sqlc.narg('position')::integer, position),
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteSubtask :exec
-- Suppression reelle : une sous-tache n'a pas d'histoire propre, et le
-- « Annuler » du rappel la recree telle quelle.
DELETE FROM subtasks WHERE id = $1;

-- name: GetSubtask :one
SELECT * FROM subtasks WHERE id = $1;

-- name: ListTaskComments :many
SELECT
    tc.*,
    u.firstname,
    u.lastname,
    u.avatar_url
FROM task_comments tc
LEFT JOIN users u ON u.id = tc.author_id
WHERE tc.task_id = $1 AND tc.deleted_at IS NULL
ORDER BY tc.created_at DESC
LIMIT sqlc.arg('page_size');

-- name: CreateTaskComment :one
INSERT INTO task_comments (task_id, author_id, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: SoftDeleteTaskComment :exec
UPDATE task_comments SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND author_id = $2 AND deleted_at IS NULL;

-- name: ListTaskActivity :many
SELECT
    ta.*,
    u.firstname,
    u.lastname,
    u.avatar_url
FROM task_activity ta
LEFT JOIN users u ON u.id = ta.actor_id
WHERE ta.task_id = $1
ORDER BY ta.created_at DESC
LIMIT sqlc.arg('page_size');

-- name: LogTaskActivity :exec
INSERT INTO task_activity (task_id, actor_id, kind, payload)
VALUES ($1, $2, $3, $4);
