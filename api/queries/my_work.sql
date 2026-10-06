-- Page « Mon travail » : ce qui attend une personne, dans tous ses projets.
--
-- Cinq requetes bornees plutot qu'une seule : chaque bloc a son ordre et sa
-- limite, et les reunir demanderait des UNION aux colonnes forcees qui ne se
-- liraient plus.

-- name: ListMyOpenTasks :many
-- Les taches non terminees d'une personne, les plus pressantes d'abord : en
-- retard, puis par echeance, puis sans echeance.
--
-- Les deux comptes portent sur toutes les lignes, pas sur la page : l'ecran
-- annonce « 12 en retard » meme quand il n'en montre que les premieres.
SELECT
    t.id,
    t.title,
    t.status,
    t.priority,
    t.due_on,
    p.id   AS project_id,
    p.name AS project_name,
    count(*) OVER () AS total,
    count(*) FILTER (WHERE t.due_on < sqlc.arg('today')::date) OVER () AS overdue
FROM task_assignees ta
JOIN tasks t    ON t.id = ta.task_id AND t.deleted_at IS NULL
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE ta.user_id = sqlc.arg('user_id')
  AND t.status <> 'done'
ORDER BY t.due_on ASC NULLS LAST, t.updated_at DESC, t.id
LIMIT sqlc.arg('page_size');

-- name: ListMyOpenTickets :many
-- Les tickets ouverts confies a une personne, les plus urgents d'abord.
SELECT
    t.id,
    t.numero,
    t.subject,
    t.status,
    t.priority,
    t.updated_at,
    p.id   AS project_id,
    p.name AS project_name,
    count(*) OVER () AS total
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.assignee_id = sqlc.arg('user_id')
  AND t.deleted_at IS NULL
  AND t.status NOT IN ('done', 'annule')
ORDER BY
    array_position(ARRAY['critical', 'urgent', 'high', 'normal', 'low'], t.priority),
    t.updated_at DESC,
    t.id
LIMIT sqlc.arg('page_size');

-- name: ListMyDeliverablesToSubmit :many
-- Les livrables a deposer dans les projets d'une personne : ceux qui n'ont
-- encore aucune version, et ceux dont le client a renvoye des retours. Ce qui
-- attend la decision du client n'y figure pas — la balle n'est pas dans le
-- camp de l'equipe.
SELECT
    d.id,
    d.title,
    p.id   AS project_id,
    p.name AS project_name,
    v.numero     AS version,
    v.decided_at AS decided_at,
    coalesce(v.feedback, '')::text AS feedback,
    count(*) OVER () AS total
FROM project_members pm
JOIN projects p     ON p.id = pm.project_id AND p.deleted_at IS NULL AND p.status NOT IN ('livre', 'hebergement')
JOIN deliverables d ON d.project_id = p.id AND d.deleted_at IS NULL
LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
WHERE pm.user_id = sqlc.arg('user_id')
  AND (d.current_version_id IS NULL OR v.decision = 'retours')
ORDER BY (v.decision = 'retours') DESC NULLS LAST, v.decided_at DESC NULLS LAST, d.created_at DESC, d.id
LIMIT sqlc.arg('page_size');

-- name: ListMyProjects :many
-- Les projets en cours ou une personne intervient, les plus proches de leur
-- echeance d'abord. Les compteurs de taches sont ceux tenus par les
-- declencheurs : aucun comptage ici.
SELECT
    p.id,
    p.name,
    p.status,
    p.progress,
    p.due_on,
    p.tasks_total,
    p.tasks_done,
    c.name AS client_name
FROM project_members pm
JOIN projects p ON p.id = pm.project_id AND p.deleted_at IS NULL AND p.status NOT IN ('livre', 'hebergement')
JOIN clients c  ON c.id = p.client_id
WHERE pm.user_id = sqlc.arg('user_id')
ORDER BY p.due_on ASC NULLS LAST, p.name
LIMIT sqlc.arg('page_size');

-- name: SumMyWeekMinutes :one
-- Le temps saisi par une personne sur une semaine. Au plus quelques dizaines
-- de lignes, lues par l'index (user_id, spent_on).
SELECT coalesce(sum(minutes), 0)::bigint
FROM time_entries
WHERE user_id = sqlc.arg('user_id')
  AND deleted_at IS NULL
  AND spent_on >= sqlc.arg('week_start')::date
  AND spent_on <  sqlc.arg('week_start')::date + 7;
