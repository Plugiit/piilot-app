-- name: GetUptimeSettings :one
SELECT * FROM uptime_settings WHERE id = 1;

-- name: UpdateUptimeSettings :exec
-- La cle d'API ne change que sur demande : un formulaire renvoye sans elle
-- garde celle qui est en base.
UPDATE uptime_settings SET
    enabled      = sqlc.arg('enabled'),
    base_url     = sqlc.arg('base_url'),
    poll_minutes = sqlc.arg('poll_minutes'),
    api_key_enc  = CASE WHEN sqlc.arg('set_api_key')::boolean THEN sqlc.narg('api_key_enc') ELSE api_key_enc END,
    updated_at   = now()
WHERE id = 1;

-- name: SetUptimeWebhookSecret :exec
UPDATE uptime_settings SET webhook_secret = sqlc.arg('secret'), updated_at = now() WHERE id = 1;

-- name: RequestUptimePoll :exec
UPDATE uptime_settings SET poll_requested_at = now() WHERE id = 1;

-- name: RecordUptimePoll :exec
UPDATE uptime_settings
SET last_poll_at = now(), last_poll_error = sqlc.narg('error'), poll_requested_at = NULL
WHERE id = 1;

-- name: TouchUptimeWebhook :exec
UPDATE uptime_settings SET webhook_last_at = now() WHERE id = 1;

-- name: UpsertUptimeMonitor :one
-- Le site tel que la releve le voit. status_changed_at ne bouge qu'au
-- changement d'etat : c'est l'heure de debut d'un incident deduit.
INSERT INTO uptime_monitors (provider, external_key, name, kind, url, hostname, status, response_ms, cert_days, cert_valid)
VALUES (sqlc.arg('provider'), sqlc.arg('external_key'), sqlc.arg('name'), sqlc.arg('kind'), sqlc.arg('url'),
        sqlc.arg('hostname'), sqlc.arg('status'), sqlc.narg('response_ms'), sqlc.narg('cert_days'), sqlc.narg('cert_valid'))
ON CONFLICT (provider, external_key) DO UPDATE SET
    name              = EXCLUDED.name,
    kind              = EXCLUDED.kind,
    url               = EXCLUDED.url,
    hostname          = EXCLUDED.hostname,
    status_changed_at = CASE WHEN uptime_monitors.status <> EXCLUDED.status THEN now() ELSE uptime_monitors.status_changed_at END,
    status            = EXCLUDED.status,
    response_ms       = EXCLUDED.response_ms,
    cert_days         = EXCLUDED.cert_days,
    cert_valid        = EXCLUDED.cert_valid,
    present           = true,
    last_seen_at      = now()
RETURNING id;

-- name: MarkUptimeMonitorsAbsent :exec
-- Les sites que la releve n'a plus vus.
UPDATE uptime_monitors SET present = false
WHERE provider = sqlc.arg('provider') AND present AND NOT (id = ANY (sqlc.arg('seen')::uuid[]));

-- name: BumpUptimeDaily :exec
-- Une releve reussie : un controle de plus pour chaque site, dans son etat.
-- Un temps de reponse a -1 est inconnu.
INSERT INTO uptime_daily AS d (monitor_id, day, checks_total, checks_up, checks_down, ping_sum, ping_count, ping_min, ping_max)
SELECT u.monitor_id, sqlc.arg('day')::date, 1, (u.status = 1)::integer, (u.status = 0)::integer,
       greatest(u.ping, 0), (u.ping >= 0)::integer, nullif(u.ping, -1), nullif(u.ping, -1)
FROM (
    -- Trois tableaux de meme longueur, alignes ligne a ligne.
    SELECT unnest(sqlc.arg('monitor_ids')::uuid[]) AS monitor_id,
           unnest(sqlc.arg('statuses')::integer[]) AS status,
           unnest(sqlc.arg('pings')::integer[])     AS ping
) u
ON CONFLICT (monitor_id, day) DO UPDATE SET
    checks_total = d.checks_total + 1,
    checks_up    = d.checks_up + EXCLUDED.checks_up,
    checks_down  = d.checks_down + EXCLUDED.checks_down,
    ping_sum     = d.ping_sum + EXCLUDED.ping_sum,
    ping_count   = d.ping_count + EXCLUDED.ping_count,
    ping_min     = least(d.ping_min, EXCLUDED.ping_min),
    ping_max     = greatest(d.ping_max, EXCLUDED.ping_max);

-- name: BumpUptimeMissing :exec
-- Une releve ratee : un controle manquant pour chaque site connu. Ni en
-- ligne, ni en panne.
INSERT INTO uptime_daily AS d (monitor_id, day, checks_total, checks_missing)
SELECT id, sqlc.arg('day')::date, 1, 1 FROM uptime_monitors WHERE provider = sqlc.arg('provider') AND present
ON CONFLICT (monitor_id, day) DO UPDATE SET
    checks_total   = d.checks_total + 1,
    checks_missing = d.checks_missing + 1;

-- name: OpenIncidentsFromPoll :exec
-- Repli quand le webhook se tait : un site hors ligne sans incident ouvert
-- en ouvre un, date de son changement d'etat.
INSERT INTO uptime_incidents (monitor_id, started_at, source)
SELECT id, status_changed_at, 'poll' FROM uptime_monitors
WHERE provider = sqlc.arg('provider') AND present AND status = 0
ON CONFLICT (monitor_id) WHERE ended_at IS NULL DO NOTHING;

-- name: CloseIncidentsFromPoll :exec
-- Et un site revenu en ligne ferme le sien.
UPDATE uptime_incidents i
SET ended_at = greatest(i.started_at, m.status_changed_at)
FROM uptime_monitors m
WHERE i.monitor_id = m.id AND i.ended_at IS NULL AND m.provider = sqlc.arg('provider') AND m.status = 1;

-- name: FindUptimeMonitorByKeys :one
-- Le site d'un evenement, par la premiere cle connue.
SELECT id, status FROM uptime_monitors
WHERE provider = sqlc.arg('provider') AND external_key = ANY (sqlc.arg('keys')::text[])
ORDER BY array_position(sqlc.arg('keys')::text[], external_key)
LIMIT 1;

-- name: SetUptimeMonitorStatus :exec
UPDATE uptime_monitors
SET status_changed_at = CASE WHEN status <> sqlc.arg('status') THEN sqlc.arg('at') ELSE status_changed_at END,
    status = sqlc.arg('status')
WHERE id = sqlc.arg('id');

-- name: OpenUptimeIncident :exec
-- La releve a pu ouvrir l'incident la premiere : l'evenement du service le
-- precise alors, avec l'heure exacte et la raison.
INSERT INTO uptime_incidents (monitor_id, started_at, message, source)
VALUES (sqlc.arg('monitor_id'), sqlc.arg('started_at'), sqlc.arg('message'), 'webhook')
ON CONFLICT (monitor_id) WHERE ended_at IS NULL DO UPDATE SET
    started_at = least(uptime_incidents.started_at, EXCLUDED.started_at),
    message    = CASE WHEN uptime_incidents.message = '' THEN EXCLUDED.message ELSE uptime_incidents.message END,
    source     = 'webhook';

-- name: CloseUptimeIncident :exec
UPDATE uptime_incidents SET ended_at = greatest(started_at, sqlc.arg('ended_at'))
WHERE monitor_id = sqlc.arg('monitor_id') AND ended_at IS NULL;

-- name: CountUptimeMonitors :one
SELECT count(*)::integer FROM uptime_monitors
WHERE sqlc.arg('q')::text = '' OR name ILIKE '%' || sqlc.arg('q') || '%' OR url ILIKE '%' || sqlc.arg('q') || '%';

-- name: CountUptimeMonitorsByState :one
-- Les compteurs de l'en-tete du module : une table bornee par l'instance
-- Kuma, quelques dizaines a quelques centaines de lignes.
SELECT
    count(*) FILTER (WHERE present)::integer                AS present,
    count(*) FILTER (WHERE present AND status = 0)::integer AS down,
    count(*) FILTER (WHERE present AND status = 1)::integer AS up
FROM uptime_monitors;

-- name: ListUptimeMonitors :many
-- Les sites et leur disponibilite sur la fenetre : la somme de trente
-- agregats journaliers au plus par site, jamais les controles eux-memes.
SELECT
    m.id, m.name, m.kind, m.url, m.hostname, m.status, m.response_ms, m.cert_days, m.cert_valid,
    m.present, m.status_changed_at, m.last_seen_at,
    coalesce(w.up, 0)::integer        AS window_up,
    coalesce(w.down, 0)::integer      AS window_down,
    coalesce(w.ping_sum, 0)::bigint   AS window_ping_sum,
    coalesce(w.ping_count, 0)::integer AS window_ping_count,
    coalesce(p.names, '{}')::text[]   AS projects
FROM uptime_monitors m
LEFT JOIN LATERAL (
    SELECT sum(d.checks_up) AS up, sum(d.checks_down) AS down, sum(d.ping_sum) AS ping_sum, sum(d.ping_count) AS ping_count
    FROM uptime_daily d WHERE d.monitor_id = m.id AND d.day >= sqlc.arg('since')::date
) w ON true
LEFT JOIN LATERAL (
    SELECT array_agg(pr.name ORDER BY pr.name) AS names
    FROM project_uptime_monitors l JOIN projects pr ON pr.id = l.project_id AND pr.deleted_at IS NULL
    WHERE l.monitor_id = m.id
) p ON true
WHERE sqlc.arg('q')::text = '' OR m.name ILIKE '%' || sqlc.arg('q') || '%' OR m.url ILIKE '%' || sqlc.arg('q') || '%'
ORDER BY m.present DESC, m.status = 0 DESC, lower(m.name)
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountUptimeIncidents :one
SELECT count(*)::integer FROM uptime_incidents
WHERE sqlc.narg('monitor_ids')::uuid[] IS NULL OR monitor_id = ANY (sqlc.narg('monitor_ids')::uuid[]);

-- name: ListUptimeIncidents :many
-- Les incidents les plus recents d'abord, tous sites ou ceux d'un projet.
SELECT i.id, i.monitor_id, m.name AS monitor_name, i.started_at, i.ended_at, i.duration_seconds, i.message, i.source
FROM uptime_incidents i
JOIN uptime_monitors m ON m.id = i.monitor_id
WHERE sqlc.narg('monitor_ids')::uuid[] IS NULL OR i.monitor_id = ANY (sqlc.narg('monitor_ids')::uuid[])
ORDER BY i.started_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListProjectUptimeMonitors :many
-- Les sites d'un projet, avec leur disponibilite sur la fenetre.
SELECT
    l.environment, m.id, m.name, m.kind, m.url, m.status, m.response_ms, m.cert_days, m.cert_valid,
    m.present, m.status_changed_at,
    coalesce(w.up, 0)::integer   AS window_up,
    coalesce(w.down, 0)::integer AS window_down
FROM project_uptime_monitors l
JOIN uptime_monitors m ON m.id = l.monitor_id
LEFT JOIN LATERAL (
    SELECT sum(d.checks_up) AS up, sum(d.checks_down) AS down
    FROM uptime_daily d WHERE d.monitor_id = m.id AND d.day >= sqlc.arg('since')::date
) w ON true
WHERE l.project_id = sqlc.arg('project_id')
ORDER BY l.environment = 'prod' DESC, lower(m.name)
LIMIT 20;

-- name: ListProjectUptimeDays :many
-- Les jours de la fenetre pour les sites d'un projet : la frise de
-- disponibilite de l'ecran.
SELECT d.monitor_id, d.day, d.checks_up, d.checks_down, d.checks_missing
FROM uptime_daily d
JOIN project_uptime_monitors l ON l.monitor_id = d.monitor_id AND l.project_id = sqlc.arg('project_id')
WHERE d.day >= sqlc.arg('since')::date
ORDER BY d.day
LIMIT 600;

-- name: DeleteProjectUptimeMonitors :exec
DELETE FROM project_uptime_monitors WHERE project_id = sqlc.arg('project_id');

-- name: InsertProjectUptimeMonitor :exec
INSERT INTO project_uptime_monitors (project_id, monitor_id, environment)
VALUES (sqlc.arg('project_id'), sqlc.arg('monitor_id'), sqlc.arg('environment'));

-- name: UptimeMonitorsExist :one
SELECT count(*)::integer FROM uptime_monitors WHERE id = ANY (sqlc.arg('ids')::uuid[]);

-- name: UptimeProjectExists :one
SELECT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.arg('id') AND deleted_at IS NULL);

-- name: PortalUptimeSites :many
-- Les sites de production surveilles des projets du client, module actif,
-- et leurs agregats depuis une date : la disponibilite du portail. Un releve
-- manque ne compte ni pour ni contre.
SELECT m.id, m.name, m.url, m.hostname, m.status, l.project_id,
       coalesce(sum(d.checks_up), 0)::bigint AS checks_up,
       coalesce(sum(d.checks_down), 0)::bigint AS checks_down
FROM project_uptime_monitors l
JOIN uptime_monitors m ON m.id = l.monitor_id AND m.present
JOIN projects p ON p.id = l.project_id AND p.deleted_at IS NULL
JOIN uptime_settings s ON s.id = 1 AND s.enabled
LEFT JOIN uptime_daily d ON d.monitor_id = m.id AND d.day >= sqlc.arg('since')::date
WHERE p.client_id = sqlc.arg('client_id') AND l.environment = 'prod'
GROUP BY m.id, m.name, m.url, m.hostname, m.status, l.project_id
ORDER BY lower(m.name), m.id, l.project_id
LIMIT 50;
