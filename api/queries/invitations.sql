-- name: CreateInvitation :one
INSERT INTO invitations (email, firstname, lastname, role, client_id, token_hash, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetOpenInvitationByEmail :one
SELECT * FROM invitations
WHERE email = $1 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: GetInvitation :one
SELECT * FROM invitations WHERE id = $1;

-- name: GetInvitationByToken :one
-- Page d'acceptation : l'invitation, qui l'a envoyee et pour quel client.
-- Rendue meme expiree ou deja acceptee : c'est l'appelant qui dit pourquoi le
-- lien ne vaut plus, et l'ecran l'explique.
SELECT
    i.*,
    coalesce(btrim(u.firstname || ' ' || u.lastname), '')::text AS inviter_name,
    coalesce(c.name, '')::text                                   AS client_name
FROM invitations i
LEFT JOIN users u   ON u.id = i.invited_by
LEFT JOIN clients c ON c.id = i.client_id
WHERE i.token_hash = $1;

-- name: ListOpenInvitations :many
-- Onglet « Invitations » : celles qui attendent une reponse, expirees
-- comprises — c'est la qu'on les renvoie. Bornee : une agence n'a pas cent
-- invitations en attente, et au-dela le probleme n'est pas l'affichage.
SELECT
    i.*,
    coalesce(btrim(u.firstname || ' ' || u.lastname), '')::text AS inviter_name,
    coalesce(c.name, '')::text                                   AS client_name
FROM invitations i
LEFT JOIN users u   ON u.id = i.invited_by
LEFT JOIN clients c ON c.id = i.client_id
WHERE i.accepted_at IS NULL AND i.revoked_at IS NULL
ORDER BY i.created_at DESC
LIMIT 100;

-- name: CountOpenInvitations :one
SELECT count(*) FROM invitations WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- name: RenewInvitation :one
-- Renvoyer une invitation remplace son jeton : l'ancien lien cesse de valoir.
UPDATE invitations
SET token_hash = $2, expires_at = $3, updated_at = now()
WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
RETURNING *;

-- name: RevokeInvitation :exec
UPDATE invitations SET revoked_at = now(), updated_at = now()
WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: AcceptInvitation :exec
UPDATE invitations SET accepted_at = now(), user_id = $2, updated_at = now()
WHERE id = $1;

-- name: CreatePasswordReset :one
INSERT INTO password_resets (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetPasswordReset :one
SELECT r.*, u.email, u.firstname, u.disabled_at AS user_disabled_at, u.deleted_at AS user_deleted_at
FROM password_resets r
JOIN users u ON u.id = r.user_id
WHERE r.token_hash = $1;

-- name: InvalidateUserPasswordResets :exec
-- Un mot de passe change rend caducs les autres liens en circulation.
UPDATE password_resets SET used_at = now()
WHERE user_id = $1 AND used_at IS NULL;

-- name: CountRecentPasswordResets :one
-- Borne par compte : un tiers qui connait une adresse ne doit pas pouvoir la
-- noyer sous les e-mails de reinitialisation.
SELECT count(*) FROM password_resets
WHERE user_id = $1 AND created_at > now() - interval '1 hour';

-- name: GetClientName :one
SELECT name FROM clients WHERE id = $1 AND deleted_at IS NULL;
