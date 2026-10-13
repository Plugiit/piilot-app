-- name: GetAnalyticsSettings :one
SELECT * FROM analytics_settings WHERE id = 1;

-- name: UpdateAnalyticsSettings :exec
-- Les secrets ne changent que sur demande. Changer d'adresse ou
-- d'identifiants oublie le jeton : il ne vaut que pour l'ancienne connexion.
UPDATE analytics_settings SET
    enabled        = sqlc.arg('enabled'),
    base_url       = sqlc.arg('base_url'),
    auth_mode      = sqlc.arg('auth_mode'),
    username       = sqlc.arg('username'),
    password_enc   = CASE WHEN sqlc.arg('set_password')::boolean THEN sqlc.narg('password_enc') ELSE password_enc END,
    api_key_enc    = CASE WHEN sqlc.arg('set_api_key')::boolean THEN sqlc.narg('api_key_enc') ELSE api_key_enc END,
    token_enc      = CASE WHEN sqlc.arg('reset_token')::boolean THEN NULL ELSE token_enc END,
    api_version    = CASE WHEN sqlc.arg('version_pinned')::boolean THEN sqlc.arg('api_version') ELSE api_version END,
    version_pinned = sqlc.arg('version_pinned'),
    cache_minutes  = sqlc.arg('cache_minutes'),
    updated_at     = now()
WHERE id = 1;

-- name: RequestAnalyticsCheck :exec
UPDATE analytics_settings SET check_requested_at = now() WHERE id = 1;

-- name: RecordAnalyticsCheck :exec
UPDATE analytics_settings
SET last_check_at = now(), last_check_error = sqlc.narg('error'), check_requested_at = NULL
WHERE id = 1;

-- name: NoteAnalyticsCheck :exec
-- Ce que la releve a remarque sans echouer, et la version qu'elle a lue.
UPDATE analytics_settings
SET last_check_note = sqlc.arg('note'),
    api_version = CASE WHEN version_pinned THEN api_version ELSE sqlc.arg('api_version') END
WHERE id = 1;

-- name: SetAnalyticsToken :exec
UPDATE analytics_settings SET token_enc = sqlc.narg('token_enc') WHERE id = 1;

-- name: UpsertAnalyticsWebsite :one
INSERT INTO analytics_websites (provider, external_id, name, domain)
VALUES (sqlc.arg('provider'), sqlc.arg('external_id'), sqlc.arg('name'), sqlc.arg('domain'))
ON CONFLICT (provider, external_id) DO UPDATE SET
    name = EXCLUDED.name, domain = EXCLUDED.domain, present = true, last_seen_at = now()
RETURNING id;

-- name: MarkAnalyticsWebsitesAbsent :exec
UPDATE analytics_websites SET present = false
WHERE provider = sqlc.arg('provider') AND present AND NOT (id = ANY (sqlc.arg('seen')::uuid[]));

-- name: CountAnalyticsWebsites :one
SELECT count(*)::integer FROM analytics_websites
WHERE sqlc.arg('q')::text = '' OR name ILIKE '%' || sqlc.arg('q') || '%' OR domain ILIKE '%' || sqlc.arg('q') || '%';

-- name: CountAnalyticsWebsitesPresent :one
SELECT count(*) FILTER (WHERE present)::integer AS present,
       (SELECT count(DISTINCT website_id) FROM project_analytics_websites)::integer AS linked
FROM analytics_websites;

-- name: ListAnalyticsWebsites :many
SELECT w.id, w.external_id, w.name, w.domain, w.present, w.last_seen_at,
       coalesce(p.names, '{}')::text[] AS projects
FROM analytics_websites w
LEFT JOIN LATERAL (
    SELECT array_agg(pr.name ORDER BY pr.name) AS names
    FROM project_analytics_websites l JOIN projects pr ON pr.id = l.project_id AND pr.deleted_at IS NULL
    WHERE l.website_id = w.id
) p ON true
WHERE sqlc.arg('q')::text = '' OR w.name ILIKE '%' || sqlc.arg('q') || '%' OR w.domain ILIKE '%' || sqlc.arg('q') || '%'
ORDER BY w.present DESC, lower(w.name)
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListProdAnalyticsWebsites :many
-- Les sites de production des projets : ceux que les clients voient, les
-- seuls que la tache tient a jour.
SELECT DISTINCT w.id, w.external_id
FROM project_analytics_websites l
JOIN analytics_websites w ON w.id = l.website_id AND w.present
JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL
WHERE l.environment = 'prod'
LIMIT 500;

-- name: ListAnalyticsCacheState :many
-- L'etat du cache des sites de production : ce qui est a rafraichir.
SELECT c.website_id, c.period_key, c.fetched_at, c.last_read_at
FROM analytics_cache c
WHERE c.website_id = ANY (sqlc.arg('website_ids')::uuid[])
LIMIT 5000;

-- name: GetAnalyticsCache :one
SELECT payload, fetched_at FROM analytics_cache WHERE website_id = sqlc.arg('website_id') AND period_key = sqlc.arg('period_key');

-- name: TouchAnalyticsCache :exec
UPDATE analytics_cache SET last_read_at = now() WHERE website_id = sqlc.arg('website_id') AND period_key = sqlc.arg('period_key');

-- name: UpsertAnalyticsCache :exec
INSERT INTO analytics_cache (website_id, period_key, payload, fetched_at)
VALUES (sqlc.arg('website_id'), sqlc.arg('period_key'), sqlc.arg('payload'), now())
ON CONFLICT (website_id, period_key) DO UPDATE SET payload = EXCLUDED.payload, fetched_at = now();

-- name: PurgeAnalyticsCache :exec
-- Les periodes personnalisees que personne ne regarde plus.
DELETE FROM analytics_cache WHERE period_key LIKE 'custom:%' AND last_read_at < sqlc.arg('before');

-- name: CreateAnalyticsRequest :exec
-- Une demande identique en attente n'est pas dupliquee.
INSERT INTO analytics_requests (website_id, period_key, requested_by)
VALUES (sqlc.arg('website_id'), sqlc.arg('period_key'), sqlc.narg('requested_by'))
ON CONFLICT (website_id, period_key) WHERE served_at IS NULL DO NOTHING;

-- name: CountAnalyticsRequestsSince :one
SELECT count(*)::integer FROM analytics_requests
WHERE requested_by = sqlc.arg('user_id') AND requested_at >= sqlc.arg('since');

-- name: PendingAnalyticsRequestExists :one
SELECT EXISTS (
    SELECT 1 FROM analytics_requests
    WHERE website_id = sqlc.arg('website_id') AND period_key = sqlc.arg('period_key') AND served_at IS NULL
);

-- name: LastAnalyticsRequestError :one
-- L'erreur de la derniere demande servie pour cette vue, s'il y en a une.
SELECT error FROM analytics_requests
WHERE website_id = sqlc.arg('website_id') AND period_key = sqlc.arg('period_key') AND served_at IS NOT NULL
ORDER BY served_at DESC
LIMIT 1;

-- name: ListPendingAnalyticsRequests :many
SELECT r.id, r.website_id, w.external_id, r.period_key
FROM analytics_requests r
JOIN analytics_websites w ON w.id = r.website_id
WHERE r.served_at IS NULL
ORDER BY r.requested_at
LIMIT sqlc.arg('lim');

-- name: ServeAnalyticsRequest :exec
UPDATE analytics_requests SET served_at = now(), error = sqlc.narg('error') WHERE id = sqlc.arg('id');

-- name: PurgeAnalyticsRequests :exec
DELETE FROM analytics_requests WHERE served_at IS NOT NULL AND served_at < sqlc.arg('before');

-- name: GetAnalyticsActive :one
SELECT visitors, fetched_at FROM analytics_active WHERE website_id = sqlc.arg('website_id');

-- name: WatchAnalyticsActive :exec
-- Un ecran regarde ce site : la tache relevera ses visiteurs du moment.
INSERT INTO analytics_active (website_id) VALUES (sqlc.arg('website_id'))
ON CONFLICT (website_id) DO UPDATE SET last_read_at = now();

-- name: ListWatchedAnalyticsActive :many
SELECT a.website_id, w.external_id
FROM analytics_active a JOIN analytics_websites w ON w.id = a.website_id AND w.present
WHERE a.last_read_at >= sqlc.arg('since') AND (a.fetched_at IS NULL OR a.fetched_at < sqlc.arg('stale'))
LIMIT 200;

-- name: SetAnalyticsActive :exec
UPDATE analytics_active SET visitors = sqlc.arg('visitors'), fetched_at = now() WHERE website_id = sqlc.arg('website_id');

-- name: ListAnalyticsSnapshotsDue :many
-- Les sites de production sans photo du mois ecoule.
SELECT DISTINCT w.id, w.external_id
FROM project_analytics_websites l
JOIN analytics_websites w ON w.id = l.website_id AND w.present
JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL
WHERE l.environment = 'prod'
  AND NOT EXISTS (SELECT 1 FROM analytics_snapshots s WHERE s.website_id = w.id AND s.month = sqlc.arg('month')::date)
LIMIT 50;

-- name: InsertAnalyticsSnapshot :exec
INSERT INTO analytics_snapshots (website_id, month, payload)
VALUES (sqlc.arg('website_id'), sqlc.arg('month'), sqlc.arg('payload'))
ON CONFLICT (website_id, month) DO NOTHING;

-- name: GetAnalyticsSnapshot :one
SELECT payload, created_at FROM analytics_snapshots WHERE website_id = sqlc.arg('website_id') AND month = sqlc.arg('month');

-- name: AnalyticsProjectExists :one
SELECT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.arg('id') AND deleted_at IS NULL);

-- name: ListProjectAnalyticsWebsites :many
SELECT l.environment, w.id, w.name, w.domain, w.present
FROM project_analytics_websites l JOIN analytics_websites w ON w.id = l.website_id
WHERE l.project_id = sqlc.arg('project_id')
ORDER BY l.environment = 'prod' DESC, lower(w.name)
LIMIT 20;

-- name: DeleteProjectAnalyticsWebsites :exec
DELETE FROM project_analytics_websites WHERE project_id = sqlc.arg('project_id');

-- name: InsertProjectAnalyticsWebsite :exec
INSERT INTO project_analytics_websites (project_id, website_id, environment)
VALUES (sqlc.arg('project_id'), sqlc.arg('website_id'), sqlc.arg('environment'));

-- name: AnalyticsWebsitesExist :one
SELECT count(*)::integer FROM analytics_websites WHERE id = ANY (sqlc.arg('ids')::uuid[]);

-- name: PortalAnalyticsWebsite :one
-- Le site de production d'un projet du client, module actif. Rien pour le
-- projet d'un autre client : c'est le controle d'acces.
SELECT w.id, w.name, w.domain
FROM project_analytics_websites l
JOIN analytics_websites w ON w.id = l.website_id
JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL
JOIN analytics_settings s ON s.id = 1 AND s.enabled
WHERE l.project_id = sqlc.arg('project_id') AND p.client_id = sqlc.arg('client_id') AND l.environment = 'prod';

-- name: PortalProjectHasStatistics :one
SELECT EXISTS (
    SELECT 1 FROM project_analytics_websites l
    JOIN analytics_settings s ON s.id = 1 AND s.enabled
    WHERE l.project_id = sqlc.arg('project_id') AND l.environment = 'prod'
);

-- name: PortalAudienceSites :many
-- Les sites de production des projets du client, module actif, avec leurs
-- trente jours en cache : le bandeau d'audience du portail. Un site lie a
-- plusieurs projets ne compte qu'une fois.
SELECT w.id, w.name, w.domain,
       array_agg(DISTINCT p.id ORDER BY p.id)::uuid[] AS project_ids,
       c.payload
FROM project_analytics_websites l
JOIN analytics_websites w ON w.id = l.website_id
JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL
JOIN analytics_settings s ON s.id = 1 AND s.enabled
JOIN analytics_cache c ON c.website_id = w.id AND c.period_key = '30d'
WHERE p.client_id = sqlc.arg('client_id') AND l.environment = 'prod'
GROUP BY w.id, w.name, w.domain, c.payload
ORDER BY lower(w.name), w.id
LIMIT 10;
