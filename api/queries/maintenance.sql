-- name: ListMaintenancePlans :many
SELECT * FROM maintenance_plans
ORDER BY active DESC, lower(name)
LIMIT 100;

-- name: GetMaintenancePlan :one
SELECT * FROM maintenance_plans WHERE id = sqlc.arg('id');

-- name: CreateMaintenancePlan :one
INSERT INTO maintenance_plans (name, included_minutes, sections, active)
VALUES (sqlc.arg('name'), sqlc.arg('included_minutes'), sqlc.arg('sections'), sqlc.arg('active'))
RETURNING *;

-- name: UpdateMaintenancePlan :one
UPDATE maintenance_plans
SET name = sqlc.arg('name'), included_minutes = sqlc.arg('included_minutes'),
    sections = sqlc.arg('sections'), active = sqlc.arg('active'), updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: MaintenancePlanInUse :one
SELECT EXISTS (SELECT 1 FROM project_maintenance WHERE plan_id = sqlc.arg('id'));

-- name: DeleteMaintenancePlan :execrows
DELETE FROM maintenance_plans WHERE id = sqlc.arg('id');

-- name: ListProjectMaintenance :many
-- Les periodes d'un projet, la plus recente d'abord.
SELECT pm.id, pm.starts_on, pm.ends_on, pl.id AS plan_id, pl.name AS plan_name, pl.included_minutes
FROM project_maintenance pm
JOIN maintenance_plans pl ON pl.id = pm.plan_id
WHERE pm.project_id = sqlc.arg('project_id')
ORDER BY pm.starts_on DESC
LIMIT 24;

-- name: CloseProjectMaintenance :exec
-- Ferme la periode ouverte la veille de day. Une periode qui n'a pas encore
-- commence disparait au lieu de finir avant de debuter.
UPDATE project_maintenance
SET ends_on = (sqlc.arg('day')::date - 1)
WHERE project_maintenance.project_id = sqlc.arg('project_id') AND ends_on IS NULL
  AND starts_on < sqlc.arg('day')::date;

-- name: DeleteFutureProjectMaintenance :exec
DELETE FROM project_maintenance
WHERE project_id = sqlc.arg('project_id') AND starts_on >= sqlc.arg('day')::date;

-- name: OpenProjectMaintenance :exec
INSERT INTO project_maintenance (project_id, plan_id, starts_on)
VALUES (sqlc.arg('project_id'), sqlc.arg('plan_id'), sqlc.arg('starts_on'));

-- name: MaintenanceProjectExists :one
SELECT EXISTS (SELECT 1 FROM projects WHERE id = sqlc.arg('id') AND deleted_at IS NULL);

-- name: CountInterventions :one
SELECT count(*)::integer
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.intervention_kind IS NOT NULL
  AND e.spent_on >= sqlc.arg('from_day')::date AND e.spent_on <= sqlc.arg('to_day')::date
  AND (sqlc.narg('project_id')::uuid IS NULL OR e.project_id = sqlc.narg('project_id'));

-- name: ListInterventions :many
-- Les interventions d'un mois, de toute l'equipe, les plus recentes d'abord.
SELECT
    e.id, e.spent_on, e.minutes, e.note, e.intervention_kind, e.client_visible, e.client_note,
    p.id AS project_id, p.name AS project_name,
    u.id AS user_id, u.firstname, u.lastname, u.avatar_url
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
JOIN users u ON u.id = e.user_id
WHERE e.deleted_at IS NULL AND e.intervention_kind IS NOT NULL
  AND e.spent_on >= sqlc.arg('from_day')::date AND e.spent_on <= sqlc.arg('to_day')::date
  AND (sqlc.narg('project_id')::uuid IS NULL OR e.project_id = sqlc.narg('project_id'))
ORDER BY e.spent_on DESC, e.created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: ListMaintenanceMonth :many
-- Le mois des projets : ceux sous formule ce mois-la, et ceux qui ont recu
-- des interventions sans en avoir. Le consomme vient de l'agregat.
SELECT
    p.id, p.name,
    pl.name AS plan_name,
    pl.included_minutes,
    coalesce(m.minutes, 0)::integer AS consumed_minutes,
    coalesce(m.visible_minutes, 0)::integer AS visible_minutes
FROM projects p
LEFT JOIN LATERAL (
    SELECT pm.plan_id FROM project_maintenance pm
    WHERE pm.project_id = p.id AND pm.starts_on <= sqlc.arg('month_end')::date
      AND (pm.ends_on IS NULL OR pm.ends_on >= sqlc.arg('month')::date)
    ORDER BY pm.starts_on DESC
    LIMIT 1
) cur ON true
LEFT JOIN maintenance_plans pl ON pl.id = cur.plan_id
LEFT JOIN maintenance_months m ON m.project_id = p.id AND m.month = sqlc.arg('month')::date
WHERE p.deleted_at IS NULL
  AND (cur.plan_id IS NOT NULL OR coalesce(m.minutes, 0) > 0)
  AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id'))
ORDER BY lower(p.name)
LIMIT 200;

-- name: PortalProjectOfClient :one
SELECT EXISTS (
    SELECT 1 FROM projects p
    WHERE p.id = sqlc.arg('project_id') AND p.client_id = sqlc.arg('client_id') AND p.deleted_at IS NULL
);

-- name: CountPortalInterventions :one
SELECT count(*)::integer FROM time_entries e
WHERE e.project_id = sqlc.arg('project_id') AND e.deleted_at IS NULL AND e.client_visible;

-- name: ListPortalInterventions :many
-- Ce que le client voit : les interventions visibles, avec la note qui lui
-- est destinee. Ni la note interne, ni qui l'a faite.
SELECT e.id, e.spent_on, e.minutes, e.intervention_kind, e.client_note
FROM time_entries e
WHERE e.project_id = sqlc.arg('project_id') AND e.deleted_at IS NULL AND e.client_visible
ORDER BY e.spent_on DESC, e.created_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: PortalProjectHasMaintenance :one
-- La page « Maintenance » du portail existe : le projet est sous formule, ou
-- il a des interventions visibles.
SELECT (EXISTS (SELECT 1 FROM project_maintenance pm WHERE pm.project_id = sqlc.arg('project_id') AND pm.ends_on IS NULL)
    OR EXISTS (SELECT 1 FROM time_entries te WHERE te.project_id = sqlc.arg('project_id') AND te.client_visible AND te.deleted_at IS NULL))::boolean AS has_maintenance;
