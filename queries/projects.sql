-- name: ListProjects :many
-- Liste paginee du back-office.
--
-- Le nom du client est joint ici plutot que ramene par une seconde requete :
-- c'est une valeur scalaire par ligne, la jointure ne multiplie rien. L'equipe,
-- elle, est une collection : elle passe par ListMembersOfProjects, qui charge
-- d'un coup les membres de toute la page.
--
-- Les compteurs de taches sont lus tels quels : ce sont des colonnes tenues par
-- declencheur, aucun COUNT n'est fait au rendu.
SELECT
    p.*,
    c.name AS client_name
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('client_id')::uuid IS NULL OR p.client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY
    -- Un seul ORDER BY parametre plutot que quatre requetes : le tri vient de
    -- l'ecran, et les colonnes possibles sont closes par le handler.
    CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('dir')::text = 'asc' THEN p.name END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('dir')::text = 'desc' THEN p.name END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'progress' AND sqlc.arg('dir')::text = 'asc' THEN p.progress END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'progress' AND sqlc.arg('dir')::text = 'desc' THEN p.progress END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'budget' AND sqlc.arg('dir')::text = 'asc' THEN p.hours_spent END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'budget' AND sqlc.arg('dir')::text = 'desc' THEN p.hours_spent END DESC,
    CASE WHEN sqlc.arg('dir')::text = 'desc' THEN p.due_on END DESC NULLS LAST,
    p.due_on ASC NULLS LAST,
    -- Depart d'egalite stable : sans lui, deux projets de meme echeance
    -- peuvent changer de place d'une page a l'autre et l'un des deux disparait.
    p.id ASC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountProjects :one
SELECT count(*) FROM projects p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('client_id')::uuid IS NULL OR p.client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: GetProject :one
SELECT
    p.*,
    c.name          AS client_name,
    c.contact_name  AS client_contact_name,
    c.contact_role  AS client_contact_role,
    c.contact_email AS client_contact_email
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.id = $1 AND p.deleted_at IS NULL;

-- name: ListMembersOfProjects :many
-- Equipes de plusieurs projets en une requete.
--
-- C'est la parade au N+1 de la liste : le handler passe les identifiants de la
-- page entiere et repartit les lignes ensuite. Une requete par projet aurait
-- fait vingt allers-retours pour afficher vingt lignes.
SELECT
    pm.project_id,
    u.id,
    u.firstname,
    u.lastname,
    u.avatar_url
FROM project_members pm
JOIN users u ON u.id = pm.user_id
WHERE pm.project_id = ANY(sqlc.arg('project_ids')::uuid[])
  AND u.deleted_at IS NULL
ORDER BY pm.project_id, u.firstname, u.lastname;

-- name: CreateProject :one
INSERT INTO projects (client_id, name, status, progress, hours_sold, starts_on, due_on, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateProject :one
-- Mise a jour partielle : chaque champ absent de la requete garde sa valeur.
-- COALESCE sur un parametre nullable dit exactement cela, et evite d'ecrire
-- une requete par champ modifiable.
UPDATE projects SET
    name       = COALESCE(sqlc.narg('name')::text, name),
    status     = COALESCE(sqlc.narg('status')::text, status),
    progress   = COALESCE(sqlc.narg('progress')::smallint, progress),
    hours_sold = COALESCE(sqlc.narg('hours_sold')::numeric, hours_sold),
    client_id  = COALESCE(sqlc.narg('client_id')::uuid, client_id),
    starts_on  = CASE WHEN sqlc.arg('clear_starts_on')::boolean THEN NULL
                      ELSE COALESCE(sqlc.narg('starts_on')::date, starts_on) END,
    due_on     = CASE WHEN sqlc.arg('clear_due_on')::boolean THEN NULL
                      ELSE COALESCE(sqlc.narg('due_on')::date, due_on) END,
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteProject :exec
UPDATE projects SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: AddProjectMember :exec
INSERT INTO project_members (project_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveProjectMember :exec
DELETE FROM project_members WHERE project_id = $1 AND user_id = $2;

-- name: GetDashboardStats :one
-- Chiffres d'en-tete du tableau de bord.
--
-- Une seule requete pour les trois tuiles, comparaison comprise : chaque
-- chiffre est donne sur les trente derniers jours et sur les trente qui
-- precedent, ce qui permet d'afficher une variation sans stocker d'historique.
--
-- Ce sont des COUNT au rendu, contrairement a la regle du projet — ils sont
-- admis ici parce qu'ils portent sur deux tables bornees (les projets et les
-- clients d'une agence, quelques centaines de lignes) et qu'ils passent par
-- les index partiels `deleted_at IS NULL`. Le jour ou ces tables grossissent,
-- c'est un instantane quotidien qu'il faudra stocker, pas un index de plus.
SELECT
    (SELECT count(*) FROM projects WHERE deleted_at IS NULL)                       AS projects_total,
    (SELECT count(*) FROM projects
     WHERE deleted_at IS NULL AND created_at >= now() - interval '30 days')        AS projects_recent,
    (SELECT count(*) FROM projects
     WHERE deleted_at IS NULL
       AND created_at >= now() - interval '60 days'
       AND created_at <  now() - interval '30 days')                               AS projects_previous,

    (SELECT count(*) FROM clients WHERE deleted_at IS NULL)                        AS clients_total,
    (SELECT count(*) FROM clients
     WHERE deleted_at IS NULL AND created_at >= now() - interval '30 days')        AS clients_recent,
    (SELECT count(*) FROM clients
     WHERE deleted_at IS NULL
       AND created_at >= now() - interval '60 days'
       AND created_at <  now() - interval '30 days')                               AS clients_previous,

    (SELECT coalesce(sum(hours_sold), 0)::numeric FROM projects WHERE deleted_at IS NULL) AS hours_total,
    (SELECT coalesce(sum(hours_sold), 0)::numeric FROM projects
     WHERE deleted_at IS NULL AND created_at >= now() - interval '30 days')        AS hours_recent,
    (SELECT coalesce(sum(hours_sold), 0)::numeric FROM projects
     WHERE deleted_at IS NULL
       AND created_at >= now() - interval '60 days'
       AND created_at <  now() - interval '30 days')                               AS hours_previous;
