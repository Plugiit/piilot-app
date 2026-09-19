-- Rapports de temps.
--
-- Contrairement a l'ecran « Saisie », ces requetes lisent le temps de toute
-- l'equipe : c'est la permission `time.read` qui en decide, pas la session.
--
-- Les sommes se font au rendu, en exception assumee a la regle des agregats
-- precalcules : un rapport porte sur une periode et des filtres choisis a la
-- volee, qu'aucun cumul stocke ne peut anticiper. Elles restent bornees — la
-- periode ne depasse pas un an, et l'index (spent_on) la parcourt seul. Un
-- cumul quotidien ne ferait rien gagner : une saisie est deja une ligne par
-- personne, projet et jour.
--
-- Une heure est facturable quand son projet n'est pas interne. Le caractere
-- se lit sur le projet a la lecture, jamais stocke sur la saisie.
--
-- Chaque requete repete le meme bloc de filtres : sqlc n'a pas de fragment
-- reutilisable, et un filtre oublie dans une seule d'entre elles ferait
-- mentir le total ou le detail.

-- name: TimeReportTotals :one
SELECT
    coalesce(sum(e.minutes), 0)::bigint                                AS minutes,
    coalesce(sum(e.minutes) FILTER (WHERE NOT p.is_internal), 0)::bigint AS billable_minutes,
    coalesce(sum(e.minutes) FILTER (WHERE p.is_internal), 0)::bigint     AS non_billable_minutes,
    count(*)                                                           AS entries,
    count(DISTINCT e.user_id)                                          AS people,
    count(DISTINCT e.project_id)                                       AS projects
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
WHERE e.deleted_at IS NULL
  AND e.spent_on >= sqlc.arg('from_day') AND e.spent_on <= sqlc.arg('to_day')
  AND (sqlc.narg('project_id')::uuid IS NULL OR e.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('user_id')::uuid    IS NULL OR e.user_id    = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('service_id')::uuid IS NULL OR e.service_id = sqlc.narg('service_id')::uuid)
  AND (sqlc.narg('client_id')::uuid  IS NULL OR p.client_id  = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('billable')::boolean IS NULL OR p.is_internal = NOT sqlc.narg('billable')::boolean);

-- name: TimeReportSeries :many
-- Temps par tranche (jour, semaine ou mois), pour le graphique d'evolution.
-- Les tranches vides n'y figurent pas : c'est le service qui connait la
-- periode et les complete.
SELECT
    date_trunc(sqlc.arg('bucket')::text, e.spent_on)::date              AS bucket,
    coalesce(sum(e.minutes) FILTER (WHERE NOT p.is_internal), 0)::bigint AS billable_minutes,
    coalesce(sum(e.minutes) FILTER (WHERE p.is_internal), 0)::bigint     AS non_billable_minutes
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
WHERE e.deleted_at IS NULL
  AND e.spent_on >= sqlc.arg('from_day') AND e.spent_on <= sqlc.arg('to_day')
  AND (sqlc.narg('project_id')::uuid IS NULL OR e.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('user_id')::uuid    IS NULL OR e.user_id    = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('service_id')::uuid IS NULL OR e.service_id = sqlc.narg('service_id')::uuid)
  AND (sqlc.narg('client_id')::uuid  IS NULL OR p.client_id  = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('billable')::boolean IS NULL OR p.is_internal = NOT sqlc.narg('billable')::boolean)
GROUP BY 1
ORDER BY 1
LIMIT 400;

-- name: TimeReportGroups :many
-- Temps regroupe par projet, personne, service ou client, le plus gros poste
-- en tete. Une seule requete pour les quatre axes : la cle et le libelle se
-- choisissent par CASE, le reste est commun.
--
-- La cle est du texte et non un uuid : le temps sans service se regroupe sous
-- une cle vide, qu'un uuid ne saurait pas porter.
SELECT
    (CASE sqlc.arg('group_by')::text
        WHEN 'user'    THEN u.id::text
        WHEN 'service' THEN coalesce(s.id::text, '')
        WHEN 'client'  THEN c.id::text
        ELSE p.id::text
    END)::text AS key,
    (CASE sqlc.arg('group_by')::text
        WHEN 'user'    THEN btrim(u.firstname || ' ' || u.lastname)
        WHEN 'service' THEN coalesce(s.name, 'Sans service')
        WHEN 'client'  THEN c.name
        ELSE p.name
    END)::text AS label,
    -- Le client sous le nom d'un projet : deux projets « Refonte du site »
    -- n'ont que lui pour se distinguer.
    (CASE sqlc.arg('group_by')::text WHEN 'project' THEN c.name ELSE '' END)::text AS detail,
    (CASE sqlc.arg('group_by')::text WHEN 'service' THEN coalesce(s.color, '') ELSE '' END)::text AS color,
    coalesce(sum(e.minutes), 0)::bigint                                  AS minutes,
    coalesce(sum(e.minutes) FILTER (WHERE NOT p.is_internal), 0)::bigint AS billable_minutes,
    coalesce(sum(e.minutes) FILTER (WHERE p.is_internal), 0)::bigint     AS non_billable_minutes,
    count(*) OVER ()                                                     AS total_groups
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
JOIN clients c  ON c.id = p.client_id
-- Sans filtre de suppression : le temps d'un compte parti reste du temps passe.
JOIN users u    ON u.id = e.user_id
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.deleted_at IS NULL
  AND e.spent_on >= sqlc.arg('from_day') AND e.spent_on <= sqlc.arg('to_day')
  AND (sqlc.narg('project_id')::uuid IS NULL OR e.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('user_id')::uuid    IS NULL OR e.user_id    = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('service_id')::uuid IS NULL OR e.service_id = sqlc.narg('service_id')::uuid)
  AND (sqlc.narg('client_id')::uuid  IS NULL OR p.client_id  = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('billable')::boolean IS NULL OR p.is_internal = NOT sqlc.narg('billable')::boolean)
GROUP BY 1, 2, 3, 4
ORDER BY minutes DESC, label
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: TimeReportEntries :many
-- Detail des saisies, la plus recente en tete. Sert la liste paginee et
-- l'export, qui la parcourt avec une borne haute.
SELECT
    e.id,
    e.spent_on,
    e.minutes,
    e.note,
    u.id                                   AS user_id,
    btrim(u.firstname || ' ' || u.lastname) AS user_name,
    p.id                                   AS project_id,
    p.name                                 AS project_name,
    NOT p.is_internal                      AS billable,
    c.name                                 AS client_name,
    t.title                                AS task_title,
    s.name                                 AS service_name,
    s.color                                AS service_color
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
JOIN clients c  ON c.id = p.client_id
JOIN users u    ON u.id = e.user_id
LEFT JOIN tasks t    ON t.id = e.task_id AND t.deleted_at IS NULL
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.deleted_at IS NULL
  AND e.spent_on >= sqlc.arg('from_day') AND e.spent_on <= sqlc.arg('to_day')
  AND (sqlc.narg('project_id')::uuid IS NULL OR e.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('user_id')::uuid    IS NULL OR e.user_id    = sqlc.narg('user_id')::uuid)
  AND (sqlc.narg('service_id')::uuid IS NULL OR e.service_id = sqlc.narg('service_id')::uuid)
  AND (sqlc.narg('client_id')::uuid  IS NULL OR p.client_id  = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('billable')::boolean IS NULL OR p.is_internal = NOT sqlc.narg('billable')::boolean)
ORDER BY e.spent_on DESC, e.created_at DESC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');
