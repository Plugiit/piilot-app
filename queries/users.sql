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

-- name: UpdateUserProfile :one
-- Mise a jour partielle du compte par son titulaire.
--
-- Ni le role ni l'etat du compte n'y figurent : ce sont des droits, ils se
-- changent depuis l'administration des comptes, pas depuis ses propres
-- reglages.
UPDATE users SET
    firstname   = COALESCE(sqlc.narg('firstname')::text, firstname),
    lastname    = COALESCE(sqlc.narg('lastname')::text, lastname),
    email       = COALESCE(sqlc.narg('email')::citext, email),
    gender      = COALESCE(sqlc.narg('gender')::text, gender),
    phone       = COALESCE(sqlc.narg('phone')::text, phone),
    address     = COALESCE(sqlc.narg('address')::text, address),
    postal_code = COALESCE(sqlc.narg('postal_code')::text, postal_code),
    city        = COALESCE(sqlc.narg('city')::text, city),
    country     = COALESCE(sqlc.narg('country')::text, country),
    updated_at  = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserPassword :exec
-- L'empreinte est calculee par l'appelant : la base ne voit jamais le mot de
-- passe en clair.
UPDATE users SET password_hash = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateUserAvatar :one
-- La photo se retire en passant NULL, d'ou un parametre nullable plutot qu'un
-- COALESCE : « absent » et « efface » doivent se distinguer.
UPDATE users SET avatar_url = sqlc.narg('avatar_url')::text, updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: AvatarURLExists :one
-- Une adresse de photo est-elle celle d'un compte ?
--
-- Le magasin de fichiers est commun aux pieces jointes et aux photos : sans
-- cette verification, l'endpoint des photos servirait n'importe quelle image
-- du magasin a qui en devinerait la cle, court-circuitant les droits du projet
-- qui la porte.
SELECT EXISTS (
    SELECT 1 FROM users WHERE avatar_url = $1 AND deleted_at IS NULL
);
