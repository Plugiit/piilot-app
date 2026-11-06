-- Liaison d'un compte Discord par code a usage unique.
--
-- Ordre des verrous, partout : le code, le membre Discord (verrou consultatif,
-- rachat seulement), le compte, la liaison, puis le compteur des evenements.
-- Un changement de role ou une desactivation concurrents attendent donc la fin
-- d'une liaison et emettent apres elle.

-- name: LockDiscordMember :exec
-- Serialise les rachats d'un meme membre Discord. Sans lui, deux rachats
-- concurrents du meme membre pour deux comptes se croisent sur l'index unique
-- et le compteur, et Postgres en tue un.
SELECT pg_advisory_xact_lock(hashtextextended('discord-member:' || sqlc.arg('discord_user_id')::text, 0));

-- name: UpsertDiscordLinkCode :exec
-- Un seul code par compte : le nouveau remplace le precedent.
INSERT INTO discord_link_codes (user_id, code_hash, expires_at)
VALUES (sqlc.arg('user_id'), sqlc.arg('code_hash'), sqlc.arg('expires_at'))
ON CONFLICT (user_id) DO UPDATE
SET code_hash = EXCLUDED.code_hash, expires_at = EXCLUDED.expires_at, created_at = now();

-- name: ConsumeDiscordLinkCode :one
-- Le rachat consomme le code quoi qu'il arrive ensuite : il ne sert qu'une fois.
DELETE FROM discord_link_codes
WHERE code_hash = $1
RETURNING user_id, expires_at;

-- name: DeleteDiscordLinkCodes :exec
DELETE FROM discord_link_codes WHERE user_id = $1;

-- name: PurgeExpiredDiscordLinkCodes :execrows
DELETE FROM discord_link_codes WHERE expires_at < now();

-- name: LockActiveUserRole :one
SELECT role FROM users
WHERE id = $1 AND deleted_at IS NULL AND disabled_at IS NULL
FOR UPDATE;

-- name: LockUser :exec
SELECT id FROM users WHERE id = $1 FOR UPDATE;

-- name: GetDiscordLinkOwner :one
-- A qui ce compte Discord est lie, et si ce compte est encore actif. Les deux
-- lignes sont verrouillees : une reactivation concurrente de ce compte attend
-- de voir si la liaison lui reste.
SELECT dl.user_id, (u.deleted_at IS NULL AND u.disabled_at IS NULL)::boolean AS active
FROM discord_links dl
JOIN users u ON u.id = dl.user_id
WHERE dl.discord_user_id = $1
FOR UPDATE;

-- name: GetDiscordLinkByUser :one
SELECT discord_user_id, linked_at FROM discord_links WHERE user_id = $1;

-- name: InsertDiscordLink :one
INSERT INTO discord_links (user_id, discord_user_id)
VALUES (sqlc.arg('user_id'), sqlc.arg('discord_user_id'))
RETURNING linked_at;

-- name: DeleteDiscordLinkByUser :one
DELETE FROM discord_links WHERE user_id = $1 RETURNING discord_user_id;

-- name: DeleteDiscordLinkByDiscordID :exec
DELETE FROM discord_links WHERE discord_user_id = $1;

-- name: GetActiveDiscordLink :one
-- Une liaison n'existe pour le bot que si son compte est actif.
SELECT dl.discord_user_id, dl.linked_at, u.role
FROM discord_links dl
JOIN users u ON u.id = dl.user_id
WHERE dl.discord_user_id = $1 AND u.deleted_at IS NULL AND u.disabled_at IS NULL;

-- name: ListActiveDiscordLinks :many
-- Parcours par curseur sur l'identifiant Discord : une liaison retiree pendant
-- le parcours ne decale pas les suivantes, comme le ferait un OFFSET.
SELECT dl.discord_user_id, dl.linked_at, u.role
FROM discord_links dl
JOIN users u ON u.id = dl.user_id
WHERE dl.discord_user_id > sqlc.arg('after') AND u.deleted_at IS NULL AND u.disabled_at IS NULL
ORDER BY dl.discord_user_id
LIMIT sqlc.arg('page_size');
