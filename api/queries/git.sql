-- name: GetGitWebhook :one
SELECT secret, updated_at FROM git_webhook WHERE id;

-- name: SetGitWebhookSecret :exec
INSERT INTO git_webhook (id, secret) VALUES (true, sqlc.arg('secret'))
ON CONFLICT (id) DO UPDATE SET secret = EXCLUDED.secret, updated_at = now();

-- name: GetProjectByRepoURL :one
SELECT * FROM projects WHERE repo_url = sqlc.arg('repo_url') AND deleted_at IS NULL LIMIT 1;

-- name: UpsertPullRequest :one
-- Une pull request se recoit plusieurs fois : ouverture, mises a jour,
-- fusion. La ligne suit le dernier etat connu ; la date de fusion, une fois
-- posee, ne s'efface pas.
INSERT INTO pull_requests (project_id, provider, number, title, url, branch, author, state, opened_at, merged_at)
VALUES (
    sqlc.arg('project_id'), sqlc.arg('provider'), sqlc.arg('number'), sqlc.arg('title'), sqlc.arg('url'),
    sqlc.arg('branch'), sqlc.arg('author'), sqlc.arg('state'), sqlc.narg('opened_at'), sqlc.narg('merged_at')
)
ON CONFLICT (project_id, provider, number) DO UPDATE SET
    title      = EXCLUDED.title,
    url        = EXCLUDED.url,
    branch     = EXCLUDED.branch,
    author     = EXCLUDED.author,
    state      = EXCLUDED.state,
    opened_at  = coalesce(pull_requests.opened_at, EXCLUDED.opened_at),
    merged_at  = coalesce(EXCLUDED.merged_at, pull_requests.merged_at),
    updated_at = now()
RETURNING *;

-- name: LinkPullRequestToTicket :exec
INSERT INTO pull_request_links (pull_request_id, ticket_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: LinkPullRequestToTask :exec
INSERT INTO pull_request_links (pull_request_id, task_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListPullRequestsOfTicket :many
SELECT pr.* FROM pull_requests pr
JOIN pull_request_links l ON l.pull_request_id = pr.id
WHERE l.ticket_id = sqlc.arg('ticket_id')
ORDER BY pr.updated_at DESC
LIMIT 20;

-- name: ListPullRequestsOfTask :many
SELECT pr.* FROM pull_requests pr
JOIN pull_request_links l ON l.pull_request_id = pr.id
WHERE l.task_id = sqlc.arg('task_id')
ORDER BY pr.updated_at DESC
LIMIT 20;

-- name: GetTicketByNumeroInProject :one
SELECT id, status FROM tickets
WHERE project_id = sqlc.arg('project_id') AND numero = sqlc.arg('numero') AND deleted_at IS NULL;

-- name: GetTaskByNumeroInProject :one
SELECT id, status FROM tasks
WHERE project_id = sqlc.arg('project_id') AND numero = sqlc.arg('numero') AND deleted_at IS NULL;

-- name: InsertDeployment :one
-- Aucune ligne rendue quand la mise en ligne est deja connue : GitHub et
-- GitLab renvoient volontiers un evenement deux fois.
INSERT INTO deployments (project_id, provider, kind, name, url, environment, deployed_at)
VALUES (sqlc.arg('project_id'), sqlc.arg('provider'), sqlc.arg('kind'), sqlc.arg('name'), sqlc.arg('url'), sqlc.arg('environment'), sqlc.arg('deployed_at'))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: ListDeploymentsOfProject :many
SELECT * FROM deployments WHERE project_id = sqlc.arg('project_id')
ORDER BY deployed_at DESC
LIMIT sqlc.arg('max_rows');

-- name: ListTicketsAwaitingDeploy :many
-- Les tickets prets a deployer dont une pull request est fusionnee : une
-- mise en ligne les clot.
SELECT DISTINCT t.id, t.numero
FROM tickets t
JOIN pull_request_links l ON l.ticket_id = t.id
JOIN pull_requests pr ON pr.id = l.pull_request_id AND pr.state = 'merged'
WHERE t.project_id = sqlc.arg('project_id') AND t.status = 'ready_to_deploy' AND t.deleted_at IS NULL;

-- name: ListOpenMilestonesOfProject :many
SELECT id, title FROM milestones
WHERE project_id = sqlc.arg('project_id') AND completed_at IS NULL
ORDER BY due_on NULLS LAST, created_at;
