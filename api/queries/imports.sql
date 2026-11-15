-- name: CreateImport :one
INSERT INTO imports (entity, file_key, file_name, size_bytes, delimiter, encoding, headers, row_count, created_by)
VALUES (
    sqlc.arg('entity'), sqlc.arg('file_key'), sqlc.arg('file_name'), sqlc.arg('size_bytes'),
    sqlc.arg('delimiter'), sqlc.arg('encoding'), sqlc.arg('headers')::text[], sqlc.arg('row_count'),
    sqlc.narg('created_by')
)
RETURNING *;

-- name: GetImport :one
SELECT * FROM imports WHERE id = $1;

-- name: ListImports :many
SELECT * FROM imports
ORDER BY created_at DESC, id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountImports :one
SELECT count(*) FROM imports;

-- name: QueueImport :one
-- Met un import en file. Seuls un import depose, ou une simulation terminee
-- dont le fichier est encore la, peuvent partir : jamais un import en cours.
-- Les compteurs repartent de zero, le rapport precedent est efface a part.
UPDATE imports
SET status = 'queued', dry_run = sqlc.arg('dry_run'), mapping = sqlc.arg('mapping'),
    created_n = 0, skipped_n = 0, error_n = 0, error = '',
    started_at = NULL, finished_at = NULL, updated_at = now()
WHERE id = sqlc.arg('id')
  AND (status = 'uploaded' OR (status = 'done' AND dry_run AND file_key <> ''))
RETURNING *;

-- name: ClaimQueuedImport :one
-- Le prochain import a traiter. Une seule instruction : le verrou
-- SKIP LOCKED suffit a ce que deux instances ne prennent pas le meme.
UPDATE imports
SET status = 'running', started_at = now(), updated_at = now()
WHERE id = (
    SELECT i.id FROM imports i
    WHERE i.status = 'queued'
    ORDER BY i.created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: TouchImport :exec
UPDATE imports SET updated_at = now() WHERE id = $1;

-- name: FailStaleImports :many
-- Un import « en cours » qui ne donne plus signe de vie : son instance s'est
-- arretee. Rejoue, il redirait des lignes deja ecrites ; l'utilisateur
-- relance, et les cles d'import evitent les doublons.
UPDATE imports
SET status = 'failed', error = 'interrupted', finished_at = now(), updated_at = now()
WHERE status = 'running' AND updated_at < now() - interval '15 minutes'
RETURNING id, file_key, dry_run;

-- name: FinishImport :exec
-- Les compteurs sont comptes dans le rapport : une seule source de verite.
UPDATE imports
SET status = sqlc.arg('status'), error = sqlc.arg('error'), file_key = sqlc.arg('file_key'),
    created_n = (SELECT count(*) FROM import_rows r WHERE r.import_id = imports.id AND r.outcome = 'created'),
    skipped_n = (SELECT count(*) FROM import_rows r WHERE r.import_id = imports.id AND r.outcome = 'skipped'),
    error_n   = (SELECT count(*) FROM import_rows r WHERE r.import_id = imports.id AND r.outcome = 'error'),
    finished_at = now(), updated_at = now()
WHERE id = sqlc.arg('id');

-- name: DeleteImport :exec
DELETE FROM imports WHERE id = $1;

-- name: DeleteImportRows :exec
DELETE FROM import_rows WHERE import_id = $1;

-- name: InsertImportRows :exec
-- Un lot de lignes de rapport, passees en tableau JSON :
-- [{"line":2,"outcome":"created","reason":"","label":"...","entity_id":""}].
INSERT INTO import_rows (import_id, line, outcome, reason, label, entity_id)
SELECT
    sqlc.arg('import_id'), (e.v->>'line')::int, e.v->>'outcome', e.v->>'reason', e.v->>'label',
    NULLIF(e.v->>'entity_id', '')::uuid
FROM jsonb_array_elements(sqlc.arg('rows')::jsonb) AS e(v)
ON CONFLICT (import_id, line) DO UPDATE
SET outcome = EXCLUDED.outcome, reason = EXCLUDED.reason, label = EXCLUDED.label, entity_id = EXCLUDED.entity_id;

-- name: ListImportRows :many
SELECT * FROM import_rows
WHERE import_id = sqlc.arg('import_id')
  AND (sqlc.narg('outcome')::text IS NULL OR outcome = sqlc.narg('outcome')::text)
ORDER BY line
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountImportRows :one
SELECT count(*) FROM import_rows
WHERE import_id = sqlc.arg('import_id')
  AND (sqlc.narg('outcome')::text IS NULL OR outcome = sqlc.narg('outcome')::text);

-- name: GetImportMapping :one
SELECT * FROM import_mappings WHERE entity = $1;

-- name: UpsertImportMapping :exec
INSERT INTO import_mappings (entity, mapping, updated_by)
VALUES (sqlc.arg('entity'), sqlc.arg('mapping'), sqlc.narg('updated_by'))
ON CONFLICT (entity) DO UPDATE
SET mapping = EXCLUDED.mapping, updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: GetImportKey :one
SELECT * FROM import_keys WHERE entity = $1 AND source_key = $2;

-- name: UpsertImportKey :exec
-- Une entite recreee apres suppression reprend sa cle : la cle designe
-- toujours l'entite vivante.
INSERT INTO import_keys (entity, source_key, entity_id, import_id)
VALUES (sqlc.arg('entity'), sqlc.arg('source_key'), sqlc.arg('entity_id'), sqlc.narg('import_id'))
ON CONFLICT (entity, source_key) DO UPDATE
SET entity_id = EXCLUDED.entity_id, import_id = EXCLUDED.import_id, created_at = now();
