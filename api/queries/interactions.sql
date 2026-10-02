-- Interactions client : le journal de la relation.

-- name: ListInteractions :many
-- Le journal, du plus recent au plus ancien, filtre par client et par genre.
-- Pagination par curseur : la liste s'allonge par le haut.
SELECT
    i.*,
    c.name       AS client_name,
    p.name       AS project_name,
    u.firstname  AS author_firstname,
    u.lastname   AS author_lastname,
    u.avatar_url AS author_avatar_url
FROM client_interactions i
JOIN clients c       ON c.id = i.client_id AND c.deleted_at IS NULL
LEFT JOIN projects p ON p.id = i.project_id AND p.deleted_at IS NULL
LEFT JOIN users u    ON u.id = i.author_id AND u.deleted_at IS NULL
WHERE (sqlc.narg('client_id')::uuid IS NULL OR i.client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('kinds')::text[] IS NULL OR i.kind = ANY (sqlc.narg('kinds')::text[]))
  AND (sqlc.narg('before')::timestamptz IS NULL OR i.occurred_at < sqlc.narg('before')::timestamptz)
ORDER BY i.occurred_at DESC, i.id DESC
LIMIT sqlc.arg('page_size');

-- name: CreateInteraction :one
INSERT INTO client_interactions (client_id, kind, body, occurred_at, author_id, project_id)
VALUES (
    sqlc.arg('client_id'), sqlc.arg('kind'), sqlc.arg('body'),
    sqlc.arg('occurred_at'), sqlc.narg('author_id'), sqlc.narg('project_id')
)
RETURNING id;

-- name: GetInteraction :one
SELECT * FROM client_interactions WHERE id = $1;

-- name: DeleteInteraction :execrows
-- Seules les saisies a la main s'effacent : un evenement est une trace.
DELETE FROM client_interactions
WHERE id = $1 AND kind IN ('note', 'call', 'meeting', 'email');

-- name: RecordProjectEvent :exec
-- Inscrit un evenement au journal du client d'un projet, dans la transaction
-- du geste. Les projets internes n'ont pas de relation client a raconter.
INSERT INTO client_interactions (client_id, kind, author_id, project_id, payload)
SELECT p.client_id, sqlc.arg('kind'), sqlc.narg('author_id'), p.id, sqlc.arg('payload')
FROM projects p
WHERE p.id = sqlc.arg('project_id') AND NOT p.is_internal;
