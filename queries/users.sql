-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 AND deleted_at IS NULL;

-- name: ListUsers :many
-- Pagination cote serveur systematique : jamais de SELECT sans LIMIT.
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('role')::text IS NULL OR role = sqlc.narg('role')::text)
ORDER BY firstname, lastname
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('role')::text IS NULL OR role = sqlc.narg('role')::text);

-- name: CreateUser :one
INSERT INTO users (email, password_hash, firstname, lastname, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: TouchUserLogin :exec
UPDATE users SET last_login_at = now(), updated_at = now()
WHERE id = $1;
