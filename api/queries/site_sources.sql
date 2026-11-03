-- Sauvegardes et domaines des sites : l'ecran du projet, le point d'appel des
-- sauvegardes, la tache de fond et le rapport mensuel y lisent.

-- name: GetSiteBackupSource :one
SELECT * FROM site_backup_sources WHERE project_id = sqlc.arg('project_id');

-- name: UpsertSiteBackupToken :exec
INSERT INTO site_backup_sources (project_id, token_hash)
VALUES (sqlc.arg('project_id'), sqlc.arg('token_hash'))
ON CONFLICT (project_id) DO UPDATE SET token_hash = EXCLUDED.token_hash, updated_at = now();

-- name: UpdateSiteBackupSource :execrows
UPDATE site_backup_sources
SET expected_hours = sqlc.arg('expected_hours'), last_restore_test_on = sqlc.narg('last_restore_test_on'), updated_at = now()
WHERE project_id = sqlc.arg('project_id');

-- name: DeleteSiteBackupSource :execrows
DELETE FROM site_backup_sources WHERE project_id = sqlc.arg('project_id');

-- name: FindSiteBackupProject :one
SELECT project_id FROM site_backup_sources WHERE token_hash = sqlc.arg('token_hash');

-- name: InsertSiteBackupRun :exec
INSERT INTO site_backup_runs (project_id, status, size_bytes, message, finished_at)
VALUES (sqlc.arg('project_id'), sqlc.arg('status'), sqlc.narg('size_bytes'), sqlc.arg('message'), sqlc.arg('finished_at'));

-- name: RecordSiteBackupSuccess :exec
-- Un appel rejoue ou en retard ne fait pas reculer la date.
UPDATE site_backup_sources
SET last_success_at = greatest(last_success_at, sqlc.arg('at')::timestamptz), updated_at = now()
WHERE project_id = sqlc.arg('project_id');

-- name: RecordSiteBackupFailure :exec
UPDATE site_backup_sources
SET last_failure_at = greatest(last_failure_at, sqlc.arg('at')::timestamptz), updated_at = now()
WHERE project_id = sqlc.arg('project_id');

-- name: ListSiteBackupRuns :many
SELECT status, size_bytes, message, finished_at
FROM site_backup_runs
WHERE project_id = sqlc.arg('project_id')
ORDER BY finished_at DESC
LIMIT 10;

-- name: ListSiteBackupsToAlert :many
-- Les sauvegardes qui manquent ou dont la derniere a echoue : le silence compte
-- comme une panne. Pas plus d'une alerte par jour pour un meme projet.
SELECT s.project_id, p.name AS project_name, s.last_success_at, s.last_failure_at
FROM site_backup_sources s
JOIN projects p ON p.id = s.project_id AND p.deleted_at IS NULL
WHERE (
        coalesce(s.last_success_at, s.created_at) < sqlc.arg('now')::timestamptz - make_interval(hours => s.expected_hours)
        OR s.last_failure_at > coalesce(s.last_success_at, to_timestamp(0))
      )
  AND (s.last_alert_at IS NULL OR s.last_alert_at < sqlc.arg('now')::timestamptz - interval '1 day')
LIMIT 200;

-- name: MarkSiteBackupAlerted :exec
UPDATE site_backup_sources SET last_alert_at = sqlc.arg('at') WHERE project_id = sqlc.arg('project_id');

-- name: PruneSiteBackupRuns :exec
DELETE FROM site_backup_runs WHERE finished_at < sqlc.arg('before');

-- name: ReportBackupCounts :one
SELECT count(*) FILTER (WHERE status = 'success')::integer AS successes,
       count(*) FILTER (WHERE status = 'failed')::integer AS failures
FROM site_backup_runs
WHERE project_id = sqlc.arg('project_id') AND finished_at >= sqlc.arg('since') AND finished_at < sqlc.arg('until');

-- name: ReportLastBackupRun :one
-- La derniere execution avant la fin du mois, reussie ou non.
SELECT status, finished_at
FROM site_backup_runs
WHERE project_id = sqlc.arg('project_id') AND finished_at < sqlc.arg('until')
ORDER BY finished_at DESC
LIMIT 1;

-- name: ReportLastBackupSuccess :one
SELECT finished_at
FROM site_backup_runs
WHERE project_id = sqlc.arg('project_id') AND status = 'success' AND finished_at < sqlc.arg('until')
ORDER BY finished_at DESC
LIMIT 1;

-- name: ListProjectDomains :many
SELECT * FROM project_domains
WHERE project_id = sqlc.arg('project_id')
ORDER BY lower(domain::text)
LIMIT 20;

-- name: CountProjectDomains :one
SELECT count(*)::integer FROM project_domains WHERE project_id = sqlc.arg('project_id');

-- name: InsertProjectDomain :one
INSERT INTO project_domains (project_id, domain)
VALUES (sqlc.arg('project_id'), sqlc.arg('domain'))
ON CONFLICT (project_id, domain) DO NOTHING
RETURNING *;

-- name: DeleteProjectDomain :execrows
DELETE FROM project_domains WHERE id = sqlc.arg('id') AND project_id = sqlc.arg('project_id');

-- name: ListDomainsToCheck :many
-- Une fois par semaine ; un domaine en erreur se retente chaque jour.
SELECT id, domain FROM project_domains
WHERE checked_at IS NULL
   OR checked_at < sqlc.arg('now')::timestamptz - interval '7 days'
   OR (last_error <> '' AND checked_at < sqlc.arg('now')::timestamptz - interval '1 day')
ORDER BY checked_at NULLS FIRST
LIMIT 20;

-- name: RecordDomainExpiry :exec
UPDATE project_domains
SET expires_at = sqlc.arg('expires_at'), checked_at = sqlc.arg('at'), last_success_at = sqlc.arg('at'), last_error = '', failure_alert_at = NULL
WHERE id = sqlc.arg('id');

-- name: RecordDomainError :exec
UPDATE project_domains SET checked_at = sqlc.arg('at'), last_error = sqlc.arg('last_error')
WHERE id = sqlc.arg('id');

-- name: ListDomainsExpiring :many
SELECT d.id, d.project_id, d.domain, d.expires_at, d.last_alert_days, p.name AS project_name
FROM project_domains d
JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL
WHERE d.expires_at IS NOT NULL
  AND (d.expires_at < sqlc.arg('now')::timestamptz + interval '61 days' OR d.last_alert_days IS NOT NULL)
ORDER BY d.expires_at
LIMIT 500;

-- name: SetDomainAlertDays :exec
UPDATE project_domains SET last_alert_days = sqlc.narg('last_alert_days') WHERE id = sqlc.arg('id');

-- name: ListDomainsFailing :many
-- Un releve qui ne reussit plus : sans lui, une expiration passerait en silence.
SELECT d.id, d.project_id, d.domain, p.name AS project_name
FROM project_domains d
JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL
WHERE d.last_error <> ''
  AND coalesce(d.last_success_at, d.created_at) < sqlc.arg('now')::timestamptz - interval '3 days'
  AND (d.failure_alert_at IS NULL OR d.failure_alert_at < sqlc.arg('now')::timestamptz - interval '7 days')
LIMIT 200;

-- name: MarkDomainFailureAlerted :exec
UPDATE project_domains SET failure_alert_at = sqlc.arg('at') WHERE id = sqlc.arg('id');

-- name: ReportMergedPullRequests :many
SELECT title, author, branch, security, merged_at
FROM pull_requests
WHERE project_id = sqlc.arg('project_id') AND merged_at >= sqlc.arg('since') AND merged_at < sqlc.arg('until')
ORDER BY merged_at
LIMIT 200;
