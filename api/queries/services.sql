-- Services : referentiel des prestations de l'agence.

-- name: ListServices :many
-- Page du tableau, par ordre alphabetique.
SELECT id, name, description, color, created_at, updated_at
FROM services
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('search')::text || '%'
       OR description ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY name, id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountServices :one
SELECT count(*)
FROM services
WHERE deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('search')::text || '%'
       OR description ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: CreateService :one
INSERT INTO services (name, description, color)
VALUES (sqlc.arg('name'), sqlc.arg('description'), sqlc.arg('color'))
RETURNING id, name, description, color, created_at, updated_at;

-- name: UpdateService :one
-- Les trois champs partent ensemble : le formulaire les montre tous, et une
-- mise a jour partielle demanderait de distinguer « vide » de « inchange ».
UPDATE services
SET name        = sqlc.arg('name'),
    description = sqlc.arg('description'),
    color       = sqlc.arg('color'),
    updated_at  = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, description, color, created_at, updated_at;

-- name: DeleteService :execrows
-- Suppression douce, comme partout : un service retire d'un referentiel a pu
-- etre porte par des donnees passees.
UPDATE services
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;
