-- Serveur d'autorisation OAuth 2.1 du connecteur Claude : demandes en attente
-- de consentement, autorisations, codes et jetons. Empreintes seulement.

-- name: CreateOAuthRequest :one
INSERT INTO oauth_requests (client_id, redirect_uri, state, code_challenge, scopes, resource, browser_hash, expires_at)
VALUES (
    sqlc.arg('client_id'), sqlc.arg('redirect_uri'), sqlc.narg('state'), sqlc.arg('code_challenge'),
    sqlc.arg('scopes')::text[], sqlc.arg('resource'), sqlc.arg('browser_hash'), sqlc.arg('expires_at')
)
RETURNING id;

-- name: GetOAuthRequest :one
-- Seulement pour le navigateur qui a lance la demande.
SELECT id, client_id, redirect_uri, state, code_challenge, scopes, resource, expires_at
FROM oauth_requests
WHERE id = sqlc.arg('id') AND browser_hash = sqlc.arg('browser_hash') AND expires_at > now();

-- name: ConsumeOAuthRequest :one
-- Une demande ne se tranche qu'une fois, et seulement depuis le navigateur qui
-- l'a lancee : la decision la retire.
DELETE FROM oauth_requests
WHERE id = sqlc.arg('id') AND browser_hash = sqlc.arg('browser_hash') AND expires_at > now()
RETURNING id, client_id, redirect_uri, state, code_challenge, scopes, resource;

-- name: CreateOAuthGrant :one
INSERT INTO oauth_grants (user_id, client_id, scopes)
VALUES (sqlc.arg('user_id'), sqlc.arg('client_id'), sqlc.arg('scopes')::text[])
RETURNING id;

-- name: CreateOAuthCode :exec
INSERT INTO oauth_codes (code_hash, grant_id, redirect_uri, code_challenge, resource, expires_at)
VALUES (
    sqlc.arg('code_hash'), sqlc.arg('grant_id'), sqlc.arg('redirect_uri'), sqlc.arg('code_challenge'),
    sqlc.arg('resource'), sqlc.arg('expires_at')
);

-- name: ConsumeOAuthCode :one
-- Usage unique : le code n'est rendu qu'une fois, a qui le presente avant
-- son expiration.
UPDATE oauth_codes SET used_at = now()
WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()
RETURNING grant_id, redirect_uri, code_challenge, resource;

-- name: GetOAuthCodeGrant :one
-- Pour reconnaitre un code rejoue : il existe, mais a deja servi.
SELECT grant_id, used_at FROM oauth_codes WHERE code_hash = $1;

-- name: GetOAuthGrant :one
-- L'autorisation et l'etat du compte, lus a l'echange d'un code ou d'un jeton.
SELECT g.id, g.user_id, g.client_id, g.scopes, g.revoked_at, u.role,
       (u.deleted_at IS NULL AND u.disabled_at IS NULL)::boolean AS active
FROM oauth_grants g
JOIN users u ON u.id = g.user_id
WHERE g.id = $1;

-- name: CreateOAuthToken :exec
INSERT INTO oauth_tokens (token_hash, grant_id, kind, resource, scopes, expires_at)
VALUES (
    sqlc.arg('token_hash'), sqlc.arg('grant_id'), sqlc.arg('kind'), sqlc.arg('resource'),
    sqlc.arg('scopes')::text[], sqlc.arg('expires_at')
);

-- name: LockOAuthRefreshToken :one
-- Verrouille le jeton le temps de le remplacer : deux renouvellements
-- simultanes passent l'un apres l'autre, et le second voit le jeton servi.
-- Les delais se comparent a l'horloge de la base, celle qui les a dates.
SELECT t.grant_id, t.resource, t.scopes,
       (t.expires_at > now())::boolean AS live,
       (t.revoked_at IS NOT NULL)::boolean AS used,
       coalesce(t.revoked_at < now() - interval '10 seconds', false)::boolean AS replayed
FROM oauth_tokens t
WHERE t.token_hash = $1 AND t.kind = 'refresh'
FOR UPDATE;

-- name: RevokeOAuthToken :exec
UPDATE oauth_tokens SET revoked_at = now()
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: GetOAuthToken :one
-- Pour la revocation (RFC 7009) : quel jeton, de quelle autorisation.
SELECT t.kind, t.grant_id, g.client_id, g.user_id
FROM oauth_tokens t
JOIN oauth_grants g ON g.id = t.grant_id
WHERE t.token_hash = $1;

-- name: AuthenticateOAuthAccess :one
-- Question posee a chaque appel de /mcp : jeton valable, autorisation en
-- cours, compte actif. Lecture par cle primaire.
SELECT t.grant_id, t.scopes, t.resource, t.expires_at, g.user_id, g.client_id, u.role
FROM oauth_tokens t
JOIN oauth_grants g ON g.id = t.grant_id
JOIN users u ON u.id = g.user_id
WHERE t.token_hash = $1
  AND t.kind = 'access'
  AND t.revoked_at IS NULL
  AND t.expires_at > now()
  AND g.revoked_at IS NULL
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL;

-- name: TouchOAuthGrant :exec
-- Au plus une ecriture par minute : l'usage se date sans couter une ecriture
-- a chaque appel.
UPDATE oauth_grants SET last_used_at = now()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');

-- name: RevokeOAuthGrant :one
UPDATE oauth_grants SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, user_id, client_id;

-- name: RevokeUserOAuthGrant :one
-- Depuis Mon compte : seulement une autorisation de ce compte.
UPDATE oauth_grants SET revoked_at = now()
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id') AND revoked_at IS NULL
RETURNING id, client_id;

-- name: RevokeAllUserOAuthGrants :exec
-- Mot de passe change : tout ce qui agissait au nom du compte s'arrete.
UPDATE oauth_grants SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: ListUserOAuthGrants :many
-- Les connexions vivantes d'un compte : celles qui peuvent encore se
-- renouveler. Quelques lignes.
SELECT g.id, g.client_id, g.scopes, g.created_at, g.last_used_at
FROM oauth_grants g
WHERE g.user_id = $1
  AND g.revoked_at IS NULL
  AND EXISTS (
      SELECT 1 FROM oauth_tokens t
      WHERE t.grant_id = g.id AND t.kind = 'refresh' AND t.revoked_at IS NULL AND t.expires_at > now()
  )
ORDER BY coalesce(g.last_used_at, g.created_at) DESC
LIMIT 50;

-- name: DeleteExpiredOAuthRequests :execrows
DELETE FROM oauth_requests WHERE expires_at < now();

-- name: DeleteExpiredOAuthCodes :execrows
DELETE FROM oauth_codes WHERE expires_at < now() - interval '1 day';

-- name: DeleteExpiredOAuthTokens :execrows
-- Un jour de marge : un jeton de renouvellement servi reste reconnaissable
-- jusqu'a son expiration, au-dela il ne prouve plus rien.
DELETE FROM oauth_tokens WHERE expires_at < now() - interval '1 day';

-- name: DeleteDeadOAuthGrants :execrows
-- Revoquees depuis un mois, ou sans plus aucun jeton : rien a montrer, rien a
-- couper.
DELETE FROM oauth_grants g
WHERE g.revoked_at < now() - interval '30 days'
   OR (g.created_at < now() - interval '1 day'
       AND NOT EXISTS (SELECT 1 FROM oauth_tokens t WHERE t.grant_id = g.id));
