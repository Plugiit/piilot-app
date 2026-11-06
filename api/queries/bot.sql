-- Acces machine du bot et flux d'evenements qu'il relit.

-- name: CreateBotKey :one
INSERT INTO bot_keys (name, key_hash)
VALUES (sqlc.arg('name'), sqlc.arg('key_hash'))
RETURNING id, name, role, created_at;

-- name: ListBotKeys :many
-- Pour la commande bot-key : quelques lignes, les actives d'abord.
SELECT id, name, role, created_at, last_used_at, revoked_at
FROM bot_keys
ORDER BY revoked_at IS NOT NULL, created_at DESC
LIMIT 200;

-- name: RevokeBotKey :one
UPDATE bot_keys SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, name;

-- name: GetActiveBotKeyByHash :one
-- Question posee par la garde a chaque appel du bot : lecture par index unique.
SELECT id, role FROM bot_keys
WHERE key_hash = $1 AND revoked_at IS NULL;

-- name: TouchBotKey :exec
-- Au plus une ecriture par minute et par cle : l'usage se date sans couter une
-- ecriture a chaque appel.
UPDATE bot_keys SET last_used_at = now()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');

-- name: HasActiveBotKey :one
SELECT EXISTS (SELECT 1 FROM bot_keys WHERE revoked_at IS NULL) AS active;

-- name: EmitBotEvent :exec
-- L'identifiant vient du compteur, verrouille jusqu'au commit : les evenements
-- deviennent visibles dans l'ordre de leurs identifiants.
WITH next AS (
    UPDATE bot_event_cursor SET last_id = last_id + 1
    RETURNING last_id
)
INSERT INTO bot_events (id, type, data)
SELECT next.last_id, sqlc.arg('type')::text, sqlc.arg('data')::jsonb FROM next;

-- name: GetBotEventCursor :one
-- last_id ne compte que les emissions validees : celle d'une transaction en
-- cours arrivera apres, dans le flux.
SELECT last_id, purged_through FROM bot_event_cursor;

-- name: ListBotEventsAfter :many
SELECT id, type, data, created_at
FROM bot_events
WHERE id > sqlc.arg('after')
ORDER BY id
LIMIT sqlc.arg('page_size');

-- name: PurgeBotEvents :one
-- Purge un prefixe du flux, jusqu'au plus recent evenement trop vieux, et
-- avance le filigrane d'autant : au-dela, rien ne manque.
WITH threshold AS (
    SELECT max(e.id) AS id FROM bot_events e WHERE e.created_at < sqlc.arg('before')::timestamptz
), gone AS (
    DELETE FROM bot_events WHERE bot_events.id <= (SELECT threshold.id FROM threshold)
    RETURNING bot_events.id
)
UPDATE bot_event_cursor
SET purged_through = GREATEST(purged_through, COALESCE((SELECT max(id) FROM gone), 0))
RETURNING purged_through;
