-- name: GetProjectGitSync :one
SELECT * FROM project_git_sync WHERE project_id = sqlc.arg('project_id');

-- name: RequestGitSync :exec
INSERT INTO project_git_sync (project_id, requested_at) VALUES (sqlc.arg('project_id'), now())
ON CONFLICT (project_id) DO UPDATE SET requested_at = now();

-- name: RequestGitSyncIfNone :exec
-- Une synchronisation est demandee, sauf si l'une l'est deja.
INSERT INTO project_git_sync (project_id, requested_at) VALUES (sqlc.arg('project_id'), now())
ON CONFLICT (project_id) DO UPDATE SET requested_at = now() WHERE project_git_sync.requested_at IS NULL;

-- name: RecordGitSync :exec
-- Un essai termine. Un succes pose last_synced_at et efface l'erreur ; un
-- echec garde le dernier succes et dit pourquoi.
INSERT INTO project_git_sync (project_id, last_attempt_at, last_synced_at, last_error)
VALUES (sqlc.arg('project_id'), now(), CASE WHEN sqlc.narg('error')::text IS NULL THEN now() END, sqlc.narg('error'))
ON CONFLICT (project_id) DO UPDATE SET
    requested_at    = NULL,
    last_attempt_at = now(),
    last_synced_at  = CASE WHEN sqlc.narg('error')::text IS NULL THEN now() ELSE project_git_sync.last_synced_at END,
    last_error      = sqlc.narg('error');

-- name: ListGitSyncTargets :many
-- Les projets a synchroniser : un depot GitHub, et une formule qui compte les
-- mises a jour sur la fenetre donnee.
SELECT DISTINCT ON (p.id) p.id AS project_id, p.repo_url,
       g.requested_at, g.last_attempt_at, g.last_synced_at
FROM project_maintenance pm
JOIN maintenance_plans pl ON pl.id = pm.plan_id AND 'updates' = ANY (pl.sections)
JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL
LEFT JOIN project_git_sync g ON g.project_id = p.id
WHERE pm.starts_on <= sqlc.arg('to_day')::date
  AND (pm.ends_on IS NULL OR pm.ends_on >= sqlc.arg('from_day')::date)
  AND (p.repo_url ILIKE 'https://github.com/%' OR p.repo_url ILIKE 'git@github.com:%' OR p.repo_url ILIKE 'github.com/%')
ORDER BY p.id, pm.starts_on DESC
LIMIT 500;

-- name: GitSyncProject :one
SELECT id, repo_url, deleted_at IS NOT NULL AS deleted FROM projects WHERE id = sqlc.arg('id');

-- name: ReportGitSyncState :one
-- Ce qu'il faut pour savoir si un brouillon doit attendre la synchronisation :
-- le depot, le dernier essai, et si un jeton est la.
SELECT p.repo_url, g.last_attempt_at,
       EXISTS (SELECT 1 FROM site_update_settings)::boolean AS has_token
FROM projects p
LEFT JOIN project_git_sync g ON g.project_id = p.id
WHERE p.id = sqlc.arg('project_id');
