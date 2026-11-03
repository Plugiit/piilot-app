-- Mises a jour des sites : la cible choisie, les lancements, la tache de fond
-- et le compte rendu de la CI.

-- name: GetSiteUpdateToken :one
SELECT github_token_enc, updated_at FROM site_update_settings WHERE id;

-- name: SetSiteUpdateToken :exec
INSERT INTO site_update_settings (id, github_token_enc) VALUES (true, sqlc.arg('github_token_enc'))
ON CONFLICT (id) DO UPDATE SET github_token_enc = EXCLUDED.github_token_enc, updated_at = now();

-- name: ClearSiteUpdateToken :exec
DELETE FROM site_update_settings WHERE id;

-- name: GetSiteUpdateTarget :one
SELECT * FROM site_update_targets WHERE project_id = sqlc.arg('project_id');

-- name: UpsertSiteUpdateTarget :exec
-- Changer ce qui decide d'un lancement fait repartir l'echeance de maintenant :
-- enregistrer une frequence ne doit rien lancer dans la minute.
INSERT INTO site_update_targets (project_id, environment, workflow, ref, repo_url, every_days, enabled)
VALUES (sqlc.arg('project_id'), sqlc.arg('environment'), sqlc.arg('workflow'), sqlc.arg('ref'), sqlc.arg('repo_url'), sqlc.narg('every_days'), sqlc.arg('enabled'))
ON CONFLICT (project_id) DO UPDATE SET
    last_requested_at = CASE
        WHEN site_update_targets.environment IS DISTINCT FROM EXCLUDED.environment
          OR site_update_targets.every_days IS DISTINCT FROM EXCLUDED.every_days
          OR site_update_targets.enabled IS DISTINCT FROM EXCLUDED.enabled
          OR site_update_targets.repo_url IS DISTINCT FROM EXCLUDED.repo_url
        THEN now() ELSE site_update_targets.last_requested_at END,
    environment = EXCLUDED.environment,
    workflow    = EXCLUDED.workflow,
    ref         = EXCLUDED.ref,
    repo_url    = EXCLUDED.repo_url,
    every_days  = EXCLUDED.every_days,
    enabled     = EXCLUDED.enabled,
    updated_at  = now();

-- name: GetProjectRepoURL :one
SELECT repo_url FROM projects WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: InsertSiteUpdateRun :one
-- Bute sur l'index des lancements actifs si un autre est en cours.
INSERT INTO site_update_runs (project_id, environment, workflow, ref, repo_url, trigger, requested_by)
VALUES (sqlc.arg('project_id'), sqlc.arg('environment'), sqlc.arg('workflow'), sqlc.arg('ref'), sqlc.arg('repo_url'), sqlc.arg('trigger'), sqlc.narg('requested_by'))
RETURNING id;

-- name: TouchSiteUpdateTarget :exec
UPDATE site_update_targets SET last_requested_at = sqlc.arg('at') WHERE project_id = sqlc.arg('project_id');

-- name: ListSiteUpdateRuns :many
SELECT id, environment, workflow, ref, status, trigger, message, run_url, requested_at, dispatched_at, finished_at
FROM site_update_runs
WHERE project_id = sqlc.arg('project_id')
ORDER BY requested_at DESC
LIMIT 10;

-- name: ListSiteUpdatesDue :many
-- Les cibles planifiees dont l'heure est venue. Un dernier lancement en echec
-- arrete la planification jusqu'a ce qu'un humain relance : on n'insiste pas
-- sur un site qu'une mise a jour vient de mettre en difficulte.
SELECT t.project_id, t.environment, t.workflow, t.ref, t.repo_url
FROM site_update_targets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.enabled AND t.every_days IS NOT NULL
  AND coalesce(t.last_requested_at, t.created_at) < sqlc.arg('now')::timestamptz - make_interval(days => t.every_days)
  AND coalesce((
        SELECT r.status FROM site_update_runs r WHERE r.project_id = t.project_id ORDER BY r.requested_at DESC LIMIT 1
      ), 'success') = 'success'
LIMIT 50;

-- name: ClaimSiteUpdateRun :one
-- Une instance a la fois : le lancement passe en « dispatching » avant l'appel,
-- et n'est jamais repris tout seul si l'instance tombe pendant.
UPDATE site_update_runs
SET status = 'dispatching', attempts = attempts + 1, callback_token_hash = sqlc.arg('callback_token_hash'), claimed_at = sqlc.arg('now')::timestamptz
WHERE id = (
    SELECT id FROM site_update_runs
    WHERE status = 'queued' AND next_attempt_at <= sqlc.arg('now')::timestamptz
    ORDER BY next_attempt_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkSiteUpdateDispatched :exec
-- Le compte rendu a pu arriver avant : il garde le dernier mot.
UPDATE site_update_runs SET status = 'dispatched', dispatched_at = sqlc.arg('at')
WHERE id = sqlc.arg('id') AND status = 'dispatching';

-- name: RetrySiteUpdateRun :exec
UPDATE site_update_runs
SET status = 'queued', next_attempt_at = sqlc.arg('next_attempt_at'), message = sqlc.arg('message'), callback_token_hash = NULL
WHERE id = sqlc.arg('id') AND status = 'dispatching';

-- name: FailSiteUpdateRun :exec
UPDATE site_update_runs
SET status = 'failed', message = sqlc.arg('message'), finished_at = sqlc.arg('at'), callback_token_hash = NULL
WHERE id = sqlc.arg('id') AND status IN ('queued', 'dispatching');

-- name: ListSiteUpdateStuck :many
-- Les lancements sans nouvelles. « dispatching » trop vieux : l'instance est
-- tombee pendant l'appel, on ne sait pas si GitHub l'a recu, donc on ne relance
-- pas. « dispatched » trop vieux : la CI ne rend pas compte.
SELECT r.id, r.project_id, r.status, r.environment, p.name AS project_name
FROM site_update_runs r
JOIN projects p ON p.id = r.project_id
WHERE (r.status = 'dispatching' AND r.claimed_at < sqlc.arg('dispatching_before')::timestamptz)
   OR (r.status = 'dispatched' AND r.dispatched_at < sqlc.arg('dispatched_before')::timestamptz)
LIMIT 50;

-- name: MarkSiteUpdateStuck :execrows
-- Le jeton reste valable : un compte rendu en retard vaut mieux que l'ignorance.
UPDATE site_update_runs
SET status = sqlc.arg('status'), message = sqlc.arg('message'), finished_at = sqlc.arg('at')
WHERE id = sqlc.arg('id') AND status IN ('dispatching', 'dispatched');

-- name: FindSiteUpdateRunByToken :one
SELECT r.id, r.project_id, r.environment, p.name AS project_name
FROM site_update_runs r
JOIN projects p ON p.id = r.project_id
WHERE r.callback_token_hash = sqlc.arg('callback_token_hash') AND r.requested_at > sqlc.arg('since');

-- name: FinishSiteUpdateRun :execrows
UPDATE site_update_runs
SET status = sqlc.arg('status'), message = sqlc.arg('message'), run_url = sqlc.arg('run_url'),
    finished_at = coalesce(finished_at, sqlc.arg('at')), callback_token_hash = NULL
WHERE id = sqlc.arg('id') AND status IN ('dispatching', 'dispatched', 'timed_out', 'failed');

-- name: GetSiteUpdateRunForDispatch :one
SELECT p.name AS project_name FROM projects p WHERE p.id = sqlc.arg('id');

-- name: ReportSiteUpdateRuns :many
-- Les lancements termines du mois, du plus ancien au plus recent.
SELECT environment, status, finished_at
FROM site_update_runs
WHERE project_id = sqlc.arg('project_id') AND finished_at >= sqlc.arg('since') AND finished_at < sqlc.arg('until')
ORDER BY finished_at
LIMIT 100;
