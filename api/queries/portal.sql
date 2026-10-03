-- Portail client.
--
-- Chaque requete part de l'appelant : son compte, son role client, son client.
-- L'isolation tient dans le SQL et non dans le code qui l'appelle — un
-- identifiant de projet ou de livrable venu d'un autre client ne rend aucune
-- ligne, exactement comme un identifiant qui n'existe pas.
--
-- Les projets internes et les projets supprimes n'existent pas pour le portail.

-- name: PortalListProjects :many
-- Les projets du client de l'appelant, les plus recemment actifs d'abord, avec
-- leur prochain jalon et leur derniere activite. Les compteurs viennent des
-- declencheurs ; les deux LATERAL sont bornes a une ligne par projet.
SELECT
    p.id,
    p.name,
    p.description,
    p.status,
    p.progress,
    p.starts_on,
    p.due_on,
    p.tasks_total,
    p.tasks_done,
    p.deliverables_pending,
    -- La jointure LATERAL rend des nuls quand il n'y a plus de jalon a venir :
    -- le titre devient vide plutot que nul, l'echeance reste nullable.
    coalesce(nm.title, '')::text AS next_milestone_title,
    nm.due_on AS next_milestone_due_on,
    greatest(p.updated_at, la.at)::timestamptz AS last_activity_at
FROM users u
JOIN projects p ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
LEFT JOIN LATERAL (
    SELECT m.title, m.due_on
    FROM milestones m
    WHERE m.project_id = p.id AND m.completed_at IS NULL
    ORDER BY m.due_on NULLS LAST, m.position
    LIMIT 1
) nm ON true
LEFT JOIN LATERAL (
    SELECT max(v.submitted_at) AS at
    FROM deliverables d
    JOIN deliverable_versions v ON v.id = d.current_version_id
    WHERE d.project_id = p.id AND d.deleted_at IS NULL
) la ON true
WHERE u.id = sqlc.arg('user_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL
ORDER BY (p.status = 'livre'), last_activity_at DESC, p.id
LIMIT 100;

-- name: PortalGetProject :one
-- Un projet du client de l'appelant. Aucune ligne s'il appartient a un autre.
SELECT
    p.id,
    p.name,
    p.description,
    p.status,
    p.progress,
    p.starts_on,
    p.due_on,
    p.tasks_total,
    p.tasks_done,
    p.deliverables_pending,
    c.name AS client_name
FROM users u
JOIN projects p ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
JOIN clients c  ON c.id = p.client_id
WHERE u.id = sqlc.arg('user_id')
  AND p.id = sqlc.arg('project_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL;

-- name: PortalListDeliverables :many
-- Les livrables soumis d'un projet. Un brouillon — rien encore de depose —
-- ne regarde pas le client.
SELECT
    d.id,
    d.title,
    d.description,
    v.decision AS status,
    v.numero   AS version_numero,
    v.url      AS version_url,
    v.attachment_id,
    v.submitted_at,
    v.decided_at,
    v.feedback,
    ms.title AS milestone_title
FROM deliverables d
JOIN deliverable_versions v ON v.id = d.current_version_id
LEFT JOIN milestones ms ON ms.id = d.milestone_id
WHERE d.project_id = sqlc.arg('project_id')
  AND d.deleted_at IS NULL
ORDER BY (v.decision = 'en_attente') DESC, v.submitted_at DESC, d.id
LIMIT 200;

-- name: PortalListSharedFiles :many
-- Les fichiers que l'agence a choisi de partager. Interne par defaut.
SELECT id, filename, content_type, size_bytes, created_at
FROM attachments
WHERE project_id = sqlc.arg('project_id') AND shared_with_client
ORDER BY created_at DESC
LIMIT 100;

-- name: PortalGetDeliverable :one
-- Un livrable soumis d'un projet du client de l'appelant.
SELECT
    d.id,
    d.title,
    d.description,
    d.current_version_id,
    p.id   AS project_id,
    p.name AS project_name,
    ms.title AS milestone_title
FROM users u
JOIN projects p     ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
JOIN deliverables d ON d.project_id = p.id AND d.deleted_at IS NULL AND d.current_version_id IS NOT NULL
LEFT JOIN milestones ms ON ms.id = d.milestone_id
WHERE u.id = sqlc.arg('user_id')
  AND d.id = sqlc.arg('deliverable_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL;

-- name: PortalGetFile :one
-- Un fichier que l'appelant peut telecharger : partage sur un de ses projets,
-- ou porte par une version d'un de ses livrables.
SELECT a.*
FROM users u
JOIN projects p ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
JOIN attachments a ON a.id = sqlc.arg('file_id')
WHERE u.id = sqlc.arg('user_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL
  AND (
      (a.project_id = p.id AND a.shared_with_client)
      OR EXISTS (
          SELECT 1
          FROM deliverable_versions v
          JOIN deliverables d ON d.id = v.deliverable_id AND d.deleted_at IS NULL
          WHERE v.attachment_id = a.id AND d.project_id = p.id
      )
  )
LIMIT 1;

-- name: ListPortalRecipientsOfProject :many
-- Les comptes du portail a prevenir pour un projet : ceux de son client,
-- actifs. Les projets internes n'ont pas de client a prevenir.
SELECT u.id, u.email, u.firstname
FROM projects p
JOIN users u ON u.client_id = p.client_id
WHERE p.id = sqlc.arg('project_id')
  AND NOT p.is_internal
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL
LIMIT 50;
