-- name: GetTimer :one
-- Le chrono d'une personne, avec de quoi le nommer a l'ecran.
SELECT
    tm.user_id, tm.project_id, tm.task_id, tm.note, tm.started_at,
    p.name  AS project_name,
    t.title AS task_title
FROM time_timers tm
JOIN projects p ON p.id = tm.project_id
LEFT JOIN tasks t ON t.id = tm.task_id AND t.deleted_at IS NULL
WHERE tm.user_id = sqlc.arg('user_id');

-- name: StartTimer :exec
-- Un chrono qui tourne deja fait echouer l'insertion : c'est a l'appelant de
-- l'arreter d'abord, et de choisir ce qu'il en fait.
INSERT INTO time_timers (user_id, project_id, task_id, note)
VALUES (sqlc.arg('user_id'), sqlc.arg('project_id'), sqlc.narg('task_id')::uuid, sqlc.arg('note'));

-- name: DeleteTimer :execrows
DELETE FROM time_timers WHERE user_id = sqlc.arg('user_id');
