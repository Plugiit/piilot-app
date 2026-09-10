-- name: ListClients :many
-- Sert le champ « Client » du formulaire de projet. Pagine comme le reste,
-- meme si une agence en compte quelques dizaines : la regle ne souffre pas
-- d'exception, sinon elle finit par etre oubliee la ou elle compte.
SELECT * FROM clients
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR name ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY name
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: GetClientByID :one
SELECT * FROM clients
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetClientByName :one
-- Recherche exacte, insensible a la casse : c'est elle qui evite de creer
-- « Novaterre » a cote de « novaterre » quand le nom est saisi a la volee.
SELECT * FROM clients
WHERE lower(name) = lower(sqlc.arg('name')::text) AND deleted_at IS NULL;

-- name: CreateClient :one
INSERT INTO clients (name, contact_name, contact_email, contact_role)
VALUES ($1, $2, $3, $4)
RETURNING *;
