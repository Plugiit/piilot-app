-- name: CreateRefreshToken :one
-- Le jeton n'est jamais stocke en clair : seul son empreinte SHA-256 entre en
-- base, si bien qu'une fuite de la table ne permet pas de rejouer une session.
INSERT INTO refresh_tokens (user_id, token_hash, user_agent, ip, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRefreshTokenWithUser :one
-- Le refresh a besoin du jeton ET de l'etat du compte pour decider. Les lire
-- en une jointure plutot qu'en deux requetes evite qu'un compte supprime entre
-- les deux obtienne un nouvel acces.
--
-- La ligne est renvoyee meme revoquee ou expiree : c'est l'appelant qui
-- tranche, parce qu'un jeton revoque presente a nouveau signale un rejeu et
-- doit declencher la revocation de toute la famille.
SELECT
    rt.id          AS token_id,
    rt.user_id     AS user_id,
    rt.expires_at  AS expires_at,
    rt.revoked_at  AS revoked_at,
    u.role         AS user_role,
    u.deleted_at   AS user_deleted_at
FROM refresh_tokens rt
JOIN users u ON u.id = rt.user_id
WHERE rt.token_hash = $1;

-- name: RevokeRefreshToken :exec
-- Idempotent : revoquer deux fois ne change pas la date de la premiere
-- revocation, ce qui garde intacte la trace d'un rejeu.
UPDATE refresh_tokens
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
-- Deconnexion de toutes les sessions : rejeu detecte, changement de mot de
-- passe, ou compte desactive.
UPDATE refresh_tokens
SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
-- Purge des jetons expires depuis assez longtemps pour ne plus rien prouver.
-- Les jetons revoques recents sont conserves : ce sont eux qui permettent de
-- reconnaitre un rejeu.
DELETE FROM refresh_tokens
WHERE expires_at < now() - sqlc.arg('retention')::interval;
