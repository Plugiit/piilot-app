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

-- name: CoolifyProjectExists :one
SELECT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.arg('id') AND deleted_at IS NULL);

-- name: ListProjectCoolifyApplications :many
-- Les applications d'un projet, avec ce que la releve en sait. Une application
-- que la releve n'a jamais vue garde son uuid pour seul nom.
SELECT l.application_uuid, l.environment,
       coalesce(a.name, l.application_uuid)::text AS name,
       coalesce(a.fqdn, '')::text AS fqdn,
       coalesce(a.present, false)::boolean AS present
FROM project_coolify_applications l
LEFT JOIN coolify_applications a ON a.uuid = l.application_uuid
WHERE l.project_id = sqlc.arg('project_id')
ORDER BY l.environment DESC, lower(coalesce(a.name, l.application_uuid)), l.application_uuid;

-- name: CountCoolifyApplicationsByUUID :one
SELECT count(*) FROM coolify_applications WHERE uuid = ANY (sqlc.arg('uuids')::text[]);

-- name: DeleteProjectCoolifyApplications :exec
DELETE FROM project_coolify_applications WHERE project_id = sqlc.arg('project_id');

-- name: InsertProjectCoolifyApplication :exec
INSERT INTO project_coolify_applications (project_id, application_uuid, environment)
VALUES (sqlc.arg('project_id'), sqlc.arg('application_uuid'), sqlc.arg('environment'));

-- name: ListCoolifyLinks :many
-- Ce que la releve des deploiements doit lire : les applications liees a un
-- projet qui existe encore.
SELECT l.project_id, l.application_uuid, l.environment, coalesce(a.fqdn, '')::text AS fqdn
FROM project_coolify_applications l
JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL
LEFT JOIN coolify_applications a ON a.uuid = l.application_uuid
ORDER BY l.application_uuid, l.project_id;

-- name: InsertCoolifyDeployment :exec
-- Idempotent : un deploiement deja releve (meme identifiant) n'est pas
-- reecrit, et ce qui a ete fige dans un rapport ne bouge pas.
INSERT INTO deployments (project_id, provider, kind, name, url, environment, deployed_at, external_id)
VALUES (sqlc.arg('project_id'), 'coolify', 'deployment', sqlc.arg('name'), sqlc.arg('url'), sqlc.arg('environment'), sqlc.arg('deployed_at'), sqlc.arg('external_id'))
ON CONFLICT DO NOTHING;

-- name: ListCoolifyDeploymentsOfProject :many
SELECT name, environment, url, deployed_at FROM deployments
WHERE project_id = sqlc.arg('project_id') AND provider = 'coolify'
ORDER BY deployed_at DESC
LIMIT sqlc.arg('max_rows');

-- name: RequestCoolifyPollIfNone :exec
-- Une releve est demandee, sauf si l'une l'est deja.
UPDATE coolify_settings SET poll_requested_at = now() WHERE id = 1 AND poll_requested_at IS NULL;
