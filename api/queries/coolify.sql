-- name: GetCoolifySettings :one
SELECT * FROM coolify_settings WHERE id = 1;

-- name: UpdateCoolifySettings :exec
-- Le jeton ne change que sur demande : un formulaire renvoye sans lui garde
-- celui qui est en base.
UPDATE coolify_settings SET
    enabled       = sqlc.arg('enabled'),
    base_url      = sqlc.arg('base_url'),
    poll_minutes  = sqlc.arg('poll_minutes'),
    api_token_enc = CASE WHEN sqlc.arg('set_api_token')::boolean THEN sqlc.narg('api_token_enc') ELSE api_token_enc END,
    updated_at    = now()
WHERE id = 1;

-- name: RequestCoolifyPoll :exec
UPDATE coolify_settings SET poll_requested_at = now() WHERE id = 1;

-- name: RecordCoolifyPoll :exec
UPDATE coolify_settings
SET last_poll_at = now(), last_poll_error = sqlc.narg('error'), poll_requested_at = NULL
WHERE id = 1;

-- name: UpsertCoolifyApplication :one
INSERT INTO coolify_applications (uuid, name, fqdn, environment_name, project_name)
VALUES (sqlc.arg('uuid'), sqlc.arg('name'), sqlc.arg('fqdn'), sqlc.arg('environment_name'), sqlc.arg('project_name'))
ON CONFLICT (uuid) DO UPDATE SET
    name             = EXCLUDED.name,
    fqdn             = EXCLUDED.fqdn,
    environment_name = EXCLUDED.environment_name,
    project_name     = EXCLUDED.project_name,
    present          = true,
    last_seen_at     = now()
RETURNING id;

-- name: MarkCoolifyApplicationsAbsent :exec
-- Les applications que la releve n'a plus vues.
UPDATE coolify_applications SET present = false
WHERE present AND NOT (id = ANY (sqlc.arg('seen')::uuid[]));

-- name: ListCoolifyApplications :many
-- Une instance compte quelques dizaines d'applications : pas de pagination.
SELECT * FROM coolify_applications
ORDER BY present DESC, lower(project_name), lower(name), uuid;

-- name: CountCoolifyApplications :one
SELECT count(*) FILTER (WHERE present) AS present, count(*) AS total FROM coolify_applications;
