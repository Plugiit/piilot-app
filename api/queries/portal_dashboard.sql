-- Tableau de bord du portail client. Chaque requete part du client de
-- l'appelant, comme le reste du portail.

-- name: PortalClientOf :one
SELECT client_id FROM users
WHERE id = sqlc.arg('user_id') AND role = 'client' AND deleted_at IS NULL AND disabled_at IS NULL;

-- name: GetClientTicketStats :one
SELECT * FROM client_ticket_stats WHERE client_id = sqlc.arg('client_id');

-- name: ListClientActivityMonths :many
SELECT month, deliverables_validated, tickets_resolved
FROM client_activity_months
WHERE client_id = sqlc.arg('client_id') AND month >= sqlc.arg('since')::date
ORDER BY month
LIMIT 24;

-- name: PortalRecentActivity :many
-- Ce qui s'est passe recemment sur les projets du client — ou sur l'un
-- d'eux —, du plus recent au plus ancien : livrables deposes et valides,
-- etapes atteintes, mises en ligne, reponses de l'agence. Chaque source est
-- bornee avant la fusion.
SELECT kind::text AS kind, at::timestamptz AS at, title::text AS title,
       project_id::uuid AS project_id, project_name::text AS project_name,
       ref_id::uuid AS ref_id, number::integer AS number
FROM (
    (SELECT 'deliverable_submitted' AS kind, v.submitted_at AS at, d.title, p.id AS project_id, p.name AS project_name,
            d.id AS ref_id, v.numero AS number
     FROM deliverable_versions v
     JOIN deliverables d ON d.id = v.deliverable_id AND d.deleted_at IS NULL
     JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
     WHERE p.client_id = sqlc.arg('client_id') AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id')::uuid)
     ORDER BY v.submitted_at DESC LIMIT 20)
    UNION ALL
    (SELECT 'deliverable_validated', v.decided_at, d.title, p.id, p.name, d.id, v.numero
     FROM deliverable_versions v
     JOIN deliverables d ON d.id = v.deliverable_id AND d.deleted_at IS NULL
     JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
     WHERE p.client_id = sqlc.arg('client_id') AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id')::uuid) AND v.decision = 'valide' AND v.decided_at IS NOT NULL
     ORDER BY v.decided_at DESC LIMIT 20)
    UNION ALL
    (SELECT 'milestone_reached', m.completed_at, m.title, p.id, p.name, m.id, 0
     FROM milestones m
     JOIN projects p ON p.id = m.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
     WHERE p.client_id = sqlc.arg('client_id') AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id')::uuid) AND m.completed_at IS NOT NULL
     ORDER BY m.completed_at DESC LIMIT 20)
    UNION ALL
    (SELECT 'deployment', dp.deployed_at, dp.name, p.id, p.name, dp.id, 0
     FROM deployments dp
     JOIN projects p ON p.id = dp.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
     WHERE p.client_id = sqlc.arg('client_id') AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id')::uuid)
     ORDER BY dp.deployed_at DESC LIMIT 20)
    UNION ALL
    (SELECT 'ticket_replied', tm.created_at, t.subject, p.id, p.name, t.id, t.numero::integer
     FROM ticket_messages tm
     JOIN tickets t ON t.id = tm.ticket_id AND t.client_visible AND t.deleted_at IS NULL
     JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
     JOIN users u ON u.id = tm.author_id AND u.role <> 'client'
     WHERE p.client_id = sqlc.arg('client_id') AND (sqlc.narg('project_id')::uuid IS NULL OR p.id = sqlc.narg('project_id')::uuid) AND NOT tm.is_internal AND tm.deleted_at IS NULL
     ORDER BY tm.created_at DESC LIMIT 20)
) feed
ORDER BY at DESC
LIMIT 20;

-- name: PortalPendingDeliverables :many
-- Les livrables qui attendent la reponse du client, du plus ancien au plus
-- recent : les actions requises.
SELECT d.id, d.title, v.numero, v.submitted_at, p.id AS project_id, p.name AS project_name
FROM deliverables d
JOIN deliverable_versions v ON v.id = d.current_version_id AND v.decision = 'en_attente'
JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
WHERE p.client_id = sqlc.arg('client_id') AND d.deleted_at IS NULL
ORDER BY v.submitted_at, d.id
LIMIT 20;

-- name: PortalProjectTickets :many
-- Les demandes visibles d'un projet du client, les plus recemment actives
-- d'abord.
SELECT t.id, t.numero, t.subject, t.status, t.created_at, t.updated_at
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL AND NOT p.is_internal
WHERE p.client_id = sqlc.arg('client_id') AND p.id = sqlc.arg('project_id')
  AND t.client_visible AND t.deleted_at IS NULL
ORDER BY t.updated_at DESC, t.id
LIMIT 10;

-- name: ListClientTicketDays :many
-- Les jours ou des demandes du client ont ete resolues, depuis une date. Les
-- jours sans rien n'ont pas de ligne : l'appelant les complete a zero.
SELECT day, resolved
FROM client_ticket_days
WHERE client_id = sqlc.arg('client_id') AND day >= sqlc.arg('since')::date
ORDER BY day
LIMIT 62;
