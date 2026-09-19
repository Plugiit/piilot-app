-- name: ListNotifications :many
-- Les notifications d'une personne, les plus recentes d'abord.
--
-- Pagination par curseur et non par offset : la liste s'allonge par le haut,
-- et un OFFSET ferait reapparaitre une ligne deja lue des qu'une notification
-- arrive pendant la lecture.
SELECT
    n.*,
    u.firstname  AS actor_firstname,
    u.lastname   AS actor_lastname,
    u.avatar_url AS actor_avatar_url
FROM notifications n
LEFT JOIN users u ON u.id = n.actor_id AND u.deleted_at IS NULL
WHERE n.user_id = sqlc.arg('user_id')
  AND (sqlc.narg('before')::timestamptz IS NULL OR n.created_at < sqlc.narg('before')::timestamptz)
ORDER BY n.created_at DESC, n.id DESC
LIMIT sqlc.arg('page_size');

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications
WHERE user_id = $1 AND read_at IS NULL;

-- name: CreateNotification :one
INSERT INTO notifications (user_id, actor_id, kind, payload, task_id, project_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: MarkNotificationRead :exec
-- Le destinataire est dans la clause : sans lui, connaitre un identifiant
-- suffirait a marquer comme lue la notification de quelqu'un d'autre.
UPDATE notifications SET read_at = now()
WHERE id = $1 AND user_id = $2 AND read_at IS NULL;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET read_at = now()
WHERE user_id = $1 AND read_at IS NULL;

-- name: ListNotificationRecipients :many
-- Qui prevenir pour un geste pose sur une tache.
--
-- Les administrateurs suivent tout ce qui se passe ; les personnes affectees a
-- la tache suivent la leur. L'auteur du geste est exclu ici plutot que dans
-- chaque appelant : personne n'a besoin d'etre prevenu de ce qu'il vient de
-- faire, et l'oubli ne se verrait qu'a l'usage.
SELECT DISTINCT u.id
FROM users u
LEFT JOIN task_assignees ta ON ta.user_id = u.id AND ta.task_id = sqlc.narg('task_id')::uuid
WHERE u.deleted_at IS NULL
  AND u.id <> sqlc.arg('actor_id')
  AND (u.role = 'admin' OR ta.user_id IS NOT NULL);
