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

-- name: GetActiveUserRole :one
-- Question posee par la garde a chaque requete authentifiee : ce compte
-- est-il toujours actif, et avec quel role ? Relue en base plutot que lue dans
-- le jeton : desactiver un compte ou changer son role prend effet a la requete
-- suivante, pas a l'expiration du jeton.
SELECT role FROM users
WHERE id = $1 AND deleted_at IS NULL AND disabled_at IS NULL;

-- name: ListAccounts :many
-- Ecran « Comptes » : les comptes de l'agence et du portail, avec le client
-- auquel un compte de portail est rattache.
SELECT
    u.id, u.email, u.firstname, u.lastname, u.role, u.avatar_url,
    u.client_id, u.disabled_at, u.last_login_at, u.created_at,
    c.name AS client_name
FROM users u
LEFT JOIN clients c ON c.id = u.client_id
WHERE u.deleted_at IS NULL
  AND (sqlc.narg('role')::text IS NULL OR u.role = sqlc.narg('role')::text)
  AND (sqlc.narg('status')::text IS NULL
       OR (sqlc.narg('status')::text = 'active' AND u.disabled_at IS NULL)
       OR (sqlc.narg('status')::text = 'disabled' AND u.disabled_at IS NOT NULL))
  AND (sqlc.narg('search')::text IS NULL
       OR (u.firstname || ' ' || u.lastname) ILIKE '%' || sqlc.narg('search')::text || '%'
       OR u.email ILIKE '%' || sqlc.narg('search')::text || '%')
-- Les comptes actifs d'abord : un compte desactive est une archive, il ne
-- doit pas s'intercaler entre deux personnes avec qui l'on travaille.
ORDER BY (u.disabled_at IS NOT NULL), u.firstname, u.lastname, u.id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountAccounts :one
SELECT count(*) FROM users u
WHERE u.deleted_at IS NULL
  AND (sqlc.narg('role')::text IS NULL OR u.role = sqlc.narg('role')::text)
  AND (sqlc.narg('status')::text IS NULL
       OR (sqlc.narg('status')::text = 'active' AND u.disabled_at IS NULL)
       OR (sqlc.narg('status')::text = 'disabled' AND u.disabled_at IS NOT NULL))
  AND (sqlc.narg('search')::text IS NULL
       OR (u.firstname || ' ' || u.lastname) ILIKE '%' || sqlc.narg('search')::text || '%'
       OR u.email ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: CountActiveAdmins :one
-- Garde-fou : il reste toujours au moins un administrateur actif. Sans lui,
-- plus personne ne pourrait gerer les comptes ni les droits.
SELECT count(*) FROM users
WHERE role = 'admin' AND deleted_at IS NULL AND disabled_at IS NULL;

-- name: SetUserRole :exec
UPDATE users SET role = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DisableUser :exec
UPDATE users SET disabled_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL AND disabled_at IS NULL;

-- name: EnableUser :exec
UPDATE users SET disabled_at = NULL, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateInvitedUser :one
INSERT INTO users (email, password_hash, firstname, lastname, role, client_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
