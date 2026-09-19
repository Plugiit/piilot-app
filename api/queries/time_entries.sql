-- Temps passe.
--
-- Chaque requete porte `user_id` en clause et non en filtre : on saisit et on
-- relit son propre temps, et l'identifiant vient de la session. Un ecran qui
-- accepterait un parametre laisserait lire le pointage de n'importe qui.

-- name: ListTimeEntries :many
-- Saisies d'une personne sur une plage de jours.
--
-- L'ecran montre une journee ou une semaine : les deux bornes disent laquelle,
-- et la meme requete sert les deux.
SELECT
    e.id,
    e.spent_on,
    e.minutes,
    e.note,
    e.created_at,
    p.id   AS project_id,
    p.name AS project_name,
    t.id    AS task_id,
    t.title AS task_title,
    s.id    AS service_id,
    s.name  AS service_name,
    s.color AS service_color
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
LEFT JOIN tasks t ON t.id = e.task_id AND t.deleted_at IS NULL
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.deleted_at IS NULL
  AND e.user_id = sqlc.arg('user_id')
  AND e.spent_on >= sqlc.arg('from_day')
  AND e.spent_on <= sqlc.arg('to_day')
ORDER BY e.spent_on DESC, e.created_at DESC
LIMIT sqlc.arg('page_size');

-- name: SumTimeEntries :one
-- Total de la plage, en minutes. Calcule par la base plutot qu'en additionnant
-- les lignes rendues : la liste est bornee, le total ne doit pas l'etre.
SELECT coalesce(sum(minutes), 0)::bigint
FROM time_entries
WHERE deleted_at IS NULL
  AND user_id = sqlc.arg('user_id')
  AND spent_on >= sqlc.arg('from_day')
  AND spent_on <= sqlc.arg('to_day');

-- name: SumTimeEntriesByDay :many
-- Total par jour de la plage, pour la barre de la semaine.
SELECT spent_on, coalesce(sum(minutes), 0)::bigint AS minutes
FROM time_entries
WHERE deleted_at IS NULL
  AND user_id = sqlc.arg('user_id')
  AND spent_on >= sqlc.arg('from_day')
  AND spent_on <= sqlc.arg('to_day')
GROUP BY spent_on
ORDER BY spent_on;

-- name: CreateTimeEntry :one
INSERT INTO time_entries (user_id, project_id, task_id, service_id, spent_on, minutes, note)
VALUES (
    sqlc.arg('user_id'), sqlc.arg('project_id'), sqlc.narg('task_id'),
    sqlc.narg('service_id'), sqlc.arg('spent_on'), sqlc.arg('minutes'), sqlc.arg('note')
)
RETURNING id;

-- name: UpdateTimeEntry :one
-- La clause sur `user_id` fait le controle d'acces : une saisie qui n'est pas
-- la sienne ne correspond a aucune ligne, et l'appelant lit un 404.
UPDATE time_entries
SET project_id = sqlc.arg('project_id'),
    task_id    = sqlc.narg('task_id'),
    service_id = sqlc.narg('service_id'),
    spent_on   = sqlc.arg('spent_on'),
    minutes    = sqlc.arg('minutes'),
    note       = sqlc.arg('note'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id') AND deleted_at IS NULL
RETURNING id;

-- name: DeleteTimeEntry :execrows
UPDATE time_entries
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id') AND deleted_at IS NULL;

-- name: GetTimeEntry :one
SELECT
    e.id,
    e.spent_on,
    e.minutes,
    e.note,
    e.created_at,
    p.id   AS project_id,
    p.name AS project_name,
    t.id    AS task_id,
    t.title AS task_title,
    s.id    AS service_id,
    s.name  AS service_name,
    s.color AS service_color
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
LEFT JOIN tasks t ON t.id = e.task_id AND t.deleted_at IS NULL
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.id = sqlc.arg('id') AND e.user_id = sqlc.arg('user_id') AND e.deleted_at IS NULL;
