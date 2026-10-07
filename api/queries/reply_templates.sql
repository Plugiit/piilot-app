-- name: ListReplyTemplates :many
SELECT * FROM ticket_reply_templates ORDER BY title, id LIMIT 200;

-- name: CreateReplyTemplate :one
INSERT INTO ticket_reply_templates (title, body, created_by)
VALUES (sqlc.arg('title'), sqlc.arg('body'), sqlc.narg('created_by'))
RETURNING *;

-- name: UpdateReplyTemplate :one
UPDATE ticket_reply_templates
SET title = sqlc.arg('title'), body = sqlc.arg('body'), updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteReplyTemplate :execrows
DELETE FROM ticket_reply_templates WHERE id = sqlc.arg('id');
