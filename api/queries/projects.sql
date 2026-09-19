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
    c.name AS client_name,
    -- L'etoile est personnelle : elle se lit pour l'appelant, pas dans l'absolu.
    EXISTS (
        SELECT 1 FROM project_favorites f
        WHERE f.project_id = p.id AND f.user_id = sqlc.arg('viewer_id')
    ) AS is_favorite
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('client_id')::uuid IS NULL OR p.client_id = sqlc.narg('client_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY
    -- Les favoris remontent avant tout le reste, quel que soit le tri demande :
    -- c'est ce que promet une etoile — epingler, pas ajouter un critere de plus
    -- qu'un changement de colonne ferait oublier. Le tri choisi s'applique
    -- ensuite, a l'interieur de chaque groupe.
    is_favorite DESC,
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
    c.name AS client_name,
    -- L'interlocuteur du projet est le contact principal de son client. Joint
    -- a gauche : un client sans contact ne doit pas faire disparaitre le
    -- projet de la fiche.
    btrim(coalesce(ct.firstname, '') || ' ' || coalesce(ct.lastname, '')) AS client_contact_name,
    coalesce(ct.role, '')                                                 AS client_contact_role,
    ct.email                                                              AS client_contact_email,
    EXISTS (
        SELECT 1 FROM project_favorites f
        WHERE f.project_id = p.id AND f.user_id = sqlc.arg('viewer_id')
    ) AS is_favorite
FROM projects p
JOIN clients c ON c.id = p.client_id
LEFT JOIN contacts ct ON ct.id = c.primary_contact_id AND ct.deleted_at IS NULL
WHERE p.id = sqlc.arg('id') AND p.deleted_at IS NULL;

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
INSERT INTO projects (client_id, name, description, status, priority, progress, hours_sold, starts_on, due_on, figma_url, prod_url, preprod_url, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateProject :one
-- Mise a jour partielle : chaque champ absent de la requete garde sa valeur.
-- COALESCE sur un parametre nullable dit exactement cela, et evite d'ecrire
-- une requete par champ modifiable.
UPDATE projects SET
    name        = COALESCE(sqlc.narg('name')::text, name),
    -- La description se vide en envoyant la chaine vide, pas en omettant le
    -- champ : COALESCE ne distingue pas « absent » de « efface », et un projet
    -- doit pouvoir perdre son resume.
    description = COALESCE(sqlc.narg('description')::text, description),
    status      = COALESCE(sqlc.narg('status')::text, status),
    priority    = COALESCE(sqlc.narg('priority')::text, priority),
    figma_url   = COALESCE(sqlc.narg('figma_url')::text, figma_url),
    prod_url    = COALESCE(sqlc.narg('prod_url')::text, prod_url),
    preprod_url = COALESCE(sqlc.narg('preprod_url')::text, preprod_url),
    progress    = COALESCE(sqlc.narg('progress')::smallint, progress),
    hours_sold  = COALESCE(sqlc.narg('hours_sold')::numeric, hours_sold),
    is_internal = COALESCE(sqlc.narg('is_internal')::boolean, is_internal),
    client_id   = COALESCE(sqlc.narg('client_id')::uuid, client_id),
    starts_on   = CASE WHEN sqlc.arg('clear_starts_on')::boolean THEN NULL
                       ELSE COALESCE(sqlc.narg('starts_on')::date, starts_on) END,
    due_on      = CASE WHEN sqlc.arg('clear_due_on')::boolean THEN NULL
                       ELSE COALESCE(sqlc.narg('due_on')::date, due_on) END,
    updated_at  = now()
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
       AND created_at <  now() - interval '30 days')                               AS hours_previous,

    -- Temps facturable : lu sur `hours_spent`, que le declencheur des saisies
    -- tient a jour. Aucune somme sur time_entries au rendu. Une heure est
    -- facturable si son projet n'est pas interne ; le budget est ce qui a ete
    -- vendu sur les projets clients.
    (SELECT coalesce(sum(hours_sold), 0)::numeric FROM projects
     WHERE deleted_at IS NULL AND NOT is_internal)                                 AS time_budget,
    (SELECT coalesce(sum(hours_spent), 0)::numeric FROM projects
     WHERE deleted_at IS NULL AND NOT is_internal)                                 AS time_billable,
    (SELECT coalesce(sum(hours_spent), 0)::numeric FROM projects
     WHERE deleted_at IS NULL AND is_internal)                                     AS time_non_billable;

-- name: ListProjectFiles :many
-- Pieces jointes d'un projet, la derniere deposee en premier.
SELECT * FROM attachments WHERE project_id = $1 ORDER BY created_at DESC;

-- name: ListTaskFiles :many
-- Pieces jointes de plusieurs taches, pour le tiroir et le tableau.
SELECT * FROM attachments
WHERE task_id = ANY(sqlc.arg('task_ids')::uuid[])
ORDER BY task_id, created_at DESC;

-- name: CreateProjectFile :one
INSERT INTO attachments (project_id, filename, content_type, size_bytes, storage_key, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: CreateTaskFile :one
INSERT INTO attachments (task_id, filename, content_type, size_bytes, storage_key, uploaded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetAttachment :one
-- Une piece jointe se lit par son seul identifiant, quel que soit son
-- proprietaire : c'est ce qui permet a un unique endpoint de telechargement
-- de servir celles des projets comme celles des taches.
SELECT * FROM attachments WHERE id = $1;

-- name: DeleteAttachment :one
-- Rend la ligne supprimee : l'appelant a besoin de sa cle de stockage pour
-- effacer le fichier du disque dans la foulee.
DELETE FROM attachments WHERE id = $1 RETURNING *;

-- name: ListFavoriteProjects :many
-- Projets etoiles par l'appelant, pour les raccourcis de la barre laterale.
--
-- Bornee en dur : c'est une liste de navigation, pas un ecran. Vingt raccourcis
-- tiennent dans un panneau, au-dela l'etoile ne trie plus rien et c'est la
-- liste des projets qu'il faut ouvrir. La regle du projet veut un LIMIT partout
-- — ici il n'a pas de page suivante, il a une fin.
SELECT
    p.id,
    p.name,
    p.status
FROM project_favorites f
JOIN projects p ON p.id = f.project_id AND p.deleted_at IS NULL
WHERE f.user_id = $1
ORDER BY p.name, p.id
LIMIT 20;

-- name: AddProjectFavorite :exec
-- Poser deux fois la meme etoile n'est pas une erreur : c'est un bouton qu'on
-- peut recliquer, pas une creation.
INSERT INTO project_favorites (user_id, project_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveProjectFavorite :exec
DELETE FROM project_favorites WHERE user_id = $1 AND project_id = $2;

-- name: ListServicesOfProjects :many
-- Services de plusieurs projets en une requete.
--
-- Meme parade au N+1 que pour les equipes : une collection ne se joint pas a la
-- liste — elle multiplierait les lignes — elle se charge d'un coup pour la page
-- entiere, et le service la repartit ensuite.
SELECT
    ps.project_id,
    s.id,
    s.name,
    s.color
FROM project_services ps
JOIN services s ON s.id = ps.service_id AND s.deleted_at IS NULL
WHERE ps.project_id = ANY(sqlc.arg('project_ids')::uuid[])
ORDER BY ps.project_id, s.name, s.id;

-- name: SetProjectServices :exec
-- Remplace les services d'un projet par la liste fournie.
--
-- Effacer puis reinserer plutot que calculer une difference : la liste est
-- courte, et le formulaire envoie toujours l'etat complet qu'il veut voir.
DELETE FROM project_services WHERE project_id = sqlc.arg('project_id');

-- name: AddProjectService :exec
INSERT INTO project_services (project_id, service_id)
VALUES (sqlc.arg('project_id'), sqlc.arg('service_id'))
ON CONFLICT DO NOTHING;
