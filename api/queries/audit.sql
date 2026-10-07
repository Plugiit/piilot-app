-- Journal d'audit : en ajout seul. Ni UPDATE ni DELETE ici, sauf la purge de
-- retention, que le declencheur n'admet que declaree.

-- name: InsertAudit :exec
INSERT INTO audit_log (actor_id, actor_email, action, target_type, target_id, ip, user_agent, details)
VALUES (
    sqlc.narg('actor_id'), sqlc.arg('actor_email'), sqlc.arg('action'), sqlc.arg('target_type'),
    sqlc.arg('target_id'), sqlc.arg('ip'), sqlc.arg('user_agent'), sqlc.arg('details')
);

-- name: ListAudit :many
SELECT * FROM audit_log
WHERE (sqlc.narg('action')::text IS NULL OR action LIKE sqlc.narg('action')::text)
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('since')::timestamptz IS NULL OR at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR at < sqlc.narg('until')::timestamptz)
  AND (sqlc.narg('search')::text IS NULL
       OR actor_email ILIKE '%' || sqlc.narg('search')::text || '%'
       OR target_id ILIKE '%' || sqlc.narg('search')::text || '%'
       OR ip ILIKE '%' || sqlc.narg('search')::text || '%'
       OR details::text ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY at DESC, id
LIMIT sqlc.arg('max_rows') OFFSET sqlc.arg('skip');

-- name: CountAudit :one
SELECT count(*)::integer FROM audit_log
WHERE (sqlc.narg('action')::text IS NULL OR action LIKE sqlc.narg('action')::text)
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('since')::timestamptz IS NULL OR at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('until')::timestamptz IS NULL OR at < sqlc.narg('until')::timestamptz)
  AND (sqlc.narg('search')::text IS NULL
       OR actor_email ILIKE '%' || sqlc.narg('search')::text || '%'
       OR target_id ILIKE '%' || sqlc.narg('search')::text || '%'
       OR ip ILIKE '%' || sqlc.narg('search')::text || '%'
       OR details::text ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: AllowAuditPurge :exec
-- A appeler dans la transaction de la purge, juste avant elle.
SELECT set_config('piilot.audit_purge', 'on', true);

-- name: PurgeAudit :execrows
DELETE FROM audit_log WHERE at < now() - make_interval(days => sqlc.arg('days')::integer);
