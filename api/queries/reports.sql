-- name: GetReportSettings :one
SELECT * FROM report_settings WHERE id = 1;

-- name: UpdateReportSettings :one
UPDATE report_settings
SET auto_send = sqlc.arg('auto_send'), send_day = sqlc.arg('send_day'), wait_days = sqlc.arg('wait_days'), updated_at = now()
WHERE id = 1
RETURNING *;

-- name: ListReportsDue :many
-- Les projets sous formule ce mois-la et sans rapport : ce qui reste a
-- generer. La formule est celle du dernier jour du mois.
SELECT DISTINCT ON (p.id) p.id AS project_id, pl.name AS plan_name, pl.included_minutes, pl.sections
FROM project_maintenance pm
JOIN maintenance_plans pl ON pl.id = pm.plan_id
JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL
WHERE pm.starts_on <= sqlc.arg('month_end')::date
  AND (pm.ends_on IS NULL OR pm.ends_on >= sqlc.arg('month')::date)
  AND NOT EXISTS (SELECT 1 FROM reports r WHERE r.project_id = p.id AND r.month = sqlc.arg('month')::date)
  AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id'))
ORDER BY p.id, pm.starts_on DESC
LIMIT 200;

-- name: GetProjectPlanForMonth :one
SELECT pl.name AS plan_name, pl.included_minutes, pl.sections
FROM project_maintenance pm
JOIN maintenance_plans pl ON pl.id = pm.plan_id
WHERE pm.project_id = sqlc.arg('project_id')
  AND pm.starts_on <= sqlc.arg('month_end')::date
  AND (pm.ends_on IS NULL OR pm.ends_on >= sqlc.arg('month')::date)
ORDER BY pm.starts_on DESC
LIMIT 1;

-- name: InsertReport :one
INSERT INTO reports (project_id, month, plan_name, included_minutes, sections, payload)
VALUES (sqlc.arg('project_id'), sqlc.arg('month'), sqlc.arg('plan_name'), sqlc.arg('included_minutes'), sqlc.arg('sections'), sqlc.arg('payload'))
ON CONFLICT (project_id, month) DO NOTHING
RETURNING id;

-- name: RegenerateReport :execrows
UPDATE reports
SET plan_name = sqlc.arg('plan_name'), included_minutes = sqlc.arg('included_minutes'), sections = sqlc.arg('sections'),
    payload = sqlc.arg('payload'), generated_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'draft';

-- name: GetReport :one
SELECT r.*, p.name AS project_name, p.client_id, c.name AS client_name
FROM reports r
JOIN projects p ON p.id = r.project_id
LEFT JOIN clients c ON c.id = p.client_id
WHERE r.id = sqlc.arg('id');

-- name: CountReports :one
SELECT count(*)::integer FROM reports r
WHERE r.month = sqlc.arg('month')::date AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'));

-- name: ListReports :many
SELECT r.id, r.project_id, p.name AS project_name, c.name AS client_name, r.month, r.status, r.plan_name,
       r.generated_at, r.validated_at, r.sent_at, (r.editorial <> '')::boolean AS has_editorial
FROM reports r
JOIN projects p ON p.id = r.project_id
LEFT JOIN clients c ON c.id = p.client_id
WHERE r.month = sqlc.arg('month')::date AND (sqlc.narg('status')::text IS NULL OR r.status = sqlc.narg('status'))
ORDER BY r.status = 'sent', lower(p.name)
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountReportsByStatus :one
SELECT count(*) FILTER (WHERE status = 'draft')::integer AS drafts,
       count(*) FILTER (WHERE status = 'validated')::integer AS validated,
       count(*) FILTER (WHERE status = 'sent')::integer AS sent
FROM reports WHERE month = sqlc.arg('month')::date;

-- name: SetReportEditorial :execrows
UPDATE reports SET editorial = sqlc.arg('editorial'), updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'draft';

-- name: ValidateReport :execrows
UPDATE reports SET status = 'validated', validated_at = now(), validated_by = sqlc.narg('user_id'), updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'draft';

-- name: UnvalidateReport :execrows
UPDATE reports SET status = 'draft', validated_at = NULL, validated_by = NULL, updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'validated';

-- name: MarkReportSent :execrows
UPDATE reports SET status = 'sent', sent_at = now(), recipients = sqlc.arg('recipients'), updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'validated';

-- name: ListReportsToSend :many
-- L'envoi automatique : les rapports du mois pas encore envoyes.
SELECT id, status FROM reports WHERE month = sqlc.arg('month')::date AND status <> 'sent' LIMIT 200;

-- name: ListPortalReports :many
-- Les rapports envoyes d'un projet : l'archive du client.
SELECT r.id, r.month, r.sent_at, r.plan_name
FROM reports r
WHERE r.project_id = sqlc.arg('project_id') AND r.status = 'sent'
ORDER BY r.month DESC
LIMIT 60;

-- name: GetPortalReport :one
-- Un rapport envoye, pour le client du projet seulement.
SELECT r.*, p.name AS project_name
FROM reports r
JOIN projects p ON p.id = r.project_id AND p.deleted_at IS NULL
WHERE r.id = sqlc.arg('id') AND r.status = 'sent' AND p.client_id = sqlc.arg('client_id');

-- name: ListProjectReportContacts :many
-- Les comptes du portail du client du projet, et s'ils sont exclus.
SELECT u.id, u.firstname, u.lastname, u.email, u.avatar_url,
       EXISTS (SELECT 1 FROM project_report_opt_outs o WHERE o.project_id = p.id AND o.user_id = u.id) AS excluded
FROM projects p
JOIN users u ON u.client_id = p.client_id AND u.role = 'client' AND u.disabled_at IS NULL
WHERE p.id = sqlc.arg('project_id')
ORDER BY lower(u.lastname), lower(u.firstname)
LIMIT 50;

-- name: ListProjectReportExtraRecipients :many
SELECT id, email FROM project_report_recipients WHERE project_id = sqlc.arg('project_id') ORDER BY email LIMIT 20;

-- name: DeleteProjectReportOptOuts :exec
DELETE FROM project_report_opt_outs WHERE project_id = sqlc.arg('project_id');

-- name: InsertProjectReportOptOut :exec
INSERT INTO project_report_opt_outs (project_id, user_id) VALUES (sqlc.arg('project_id'), sqlc.arg('user_id'))
ON CONFLICT DO NOTHING;

-- name: DeleteProjectReportRecipients :exec
DELETE FROM project_report_recipients WHERE project_id = sqlc.arg('project_id');

-- name: InsertProjectReportRecipient :exec
INSERT INTO project_report_recipients (project_id, email) VALUES (sqlc.arg('project_id'), sqlc.arg('email'))
ON CONFLICT DO NOTHING;

-- name: ReportProjectExists :one
SELECT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.arg('id') AND deleted_at IS NULL);

-- name: ReportUptimeSites :many
-- Les sites du projet et leur mois : disponibilite et temps de reponse,
-- sommes des agregats journaliers du mois.
SELECT l.environment, m.id, m.name, m.url, m.cert_days, m.cert_valid,
       coalesce(sum(d.checks_up), 0)::integer   AS up,
       coalesce(sum(d.checks_down), 0)::integer AS down,
       coalesce(sum(d.ping_sum), 0)::bigint     AS ping_sum,
       coalesce(sum(d.ping_count), 0)::integer  AS ping_count
FROM project_uptime_monitors l
JOIN uptime_monitors m ON m.id = l.monitor_id
LEFT JOIN uptime_daily d ON d.monitor_id = m.id AND d.day >= sqlc.arg('from_day')::date AND d.day <= sqlc.arg('to_day')::date
WHERE l.project_id = sqlc.arg('project_id')
GROUP BY l.environment, m.id
ORDER BY l.environment = 'prod' DESC, lower(m.name)
LIMIT 20;

-- name: ReportUptimeDays :many
SELECT d.monitor_id, d.day, d.checks_up, d.checks_down
FROM uptime_daily d
JOIN project_uptime_monitors l ON l.monitor_id = d.monitor_id AND l.project_id = sqlc.arg('project_id')
WHERE d.day >= sqlc.arg('from_day')::date AND d.day <= sqlc.arg('to_day')::date
LIMIT 700;

-- name: ReportIncidents :many
-- Les incidents qui touchent le mois, meme commences avant.
SELECT i.monitor_id, m.name AS monitor_name, i.started_at, i.ended_at, i.duration_seconds, i.message
FROM uptime_incidents i
JOIN uptime_monitors m ON m.id = i.monitor_id
JOIN project_uptime_monitors l ON l.monitor_id = i.monitor_id AND l.project_id = sqlc.arg('project_id')
WHERE i.started_at < sqlc.arg('until') AND (i.ended_at IS NULL OR i.ended_at >= sqlc.arg('since'))
ORDER BY i.started_at
LIMIT 50;

-- name: ReportAudienceSnapshot :one
-- La photo figee du mois du site de production du projet.
SELECT s.payload, w.name AS website_name, w.domain
FROM project_analytics_websites l
JOIN analytics_websites w ON w.id = l.website_id
JOIN analytics_snapshots s ON s.website_id = w.id AND s.month = sqlc.arg('month')::date
WHERE l.project_id = sqlc.arg('project_id') AND l.environment = 'prod';

-- name: ReportHasAudienceSite :one
SELECT EXISTS (SELECT 1 FROM project_analytics_websites WHERE project_id = sqlc.arg('project_id') AND environment = 'prod');

-- name: ReportInterventions :many
-- Les interventions visibles du mois, avec la note destinee au client.
SELECT spent_on, minutes, intervention_kind, client_note
FROM time_entries
WHERE project_id = sqlc.arg('project_id') AND deleted_at IS NULL AND client_visible
  AND spent_on >= sqlc.arg('from_day')::date AND spent_on <= sqlc.arg('to_day')::date
ORDER BY spent_on, created_at
LIMIT 200;

-- name: ReportConsumed :one
SELECT coalesce((SELECT minutes FROM maintenance_months WHERE project_id = sqlc.arg('project_id') AND month = sqlc.arg('month')::date), 0)::integer;

-- name: ReportDeployments :many
SELECT name, environment, url, deployed_at
FROM deployments
WHERE project_id = sqlc.arg('project_id') AND deployed_at >= sqlc.arg('since') AND deployed_at < sqlc.arg('until')
ORDER BY deployed_at
LIMIT 50;
