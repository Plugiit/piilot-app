-- name: ListClients :many
-- Sert le champ « Client » du formulaire de projet. Pagine comme le reste,
-- meme si une agence en compte quelques dizaines : la regle ne souffre pas
-- d'exception, sinon elle finit par etre oubliee la ou elle compte.
--
-- L'interlocuteur affiche est le contact principal, joint a gauche : un client
-- sans contact reste proposable.
SELECT
    c.*,
    btrim(coalesce(ct.firstname, '') || ' ' || coalesce(ct.lastname, '')) AS contact_name,
    coalesce(ct.role, '')                                                 AS contact_role
FROM clients c
LEFT JOIN contacts ct ON ct.id = c.primary_contact_id AND ct.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND (sqlc.narg('search')::text IS NULL OR c.name ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY c.name
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: GetClientByID :one
SELECT * FROM clients
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetClientByName :one
-- Recherche exacte, insensible a la casse : c'est elle qui evite de creer
-- « Novaterre » a cote de « novaterre » quand le nom est saisi a la volee.
SELECT * FROM clients
WHERE lower(name) = lower(sqlc.arg('name')::text) AND deleted_at IS NULL;

-- name: CreateClient :one
-- Le client naît sans interlocuteur : ses contacts sont crees ensuite, et la
-- cle etrangere composite exige qu'un contact principal lui appartienne deja.
INSERT INTO clients (name)
VALUES ($1)
RETURNING *;

-- name: ListCrmClients :many
-- Liste paginee de l'ecran CRM.
--
-- Distincte de ListClients, qui sert le champ « Client » d'un formulaire :
-- celle-la remplit un tableau, avec sa recherche, son filtre et son tri. Les
-- confondre reviendrait a faire porter a un menu deroulant les besoins d'un
-- ecran, et inversement.
--
-- `projects_active` et `portal_users` sont lus tels quels : colonnes tenues
-- par declencheur, aucun COUNT au rendu. `contacts_count` en est un vrai, mais
-- sur un index partiel et pour la seule page affichee.
SELECT
    c.*,
    ct.id                 AS contact_id,
    coalesce(ct.firstname, '') AS contact_firstname,
    coalesce(ct.lastname, '')  AS contact_lastname,
    coalesce(ct.role, '')      AS contact_role,
    ct.email              AS contact_email,
    am.firstname          AS manager_firstname,
    am.lastname           AS manager_lastname,
    am.avatar_url         AS manager_avatar_url,
    (
        SELECT count(*) FROM contacts x
        WHERE x.client_id = c.id AND x.deleted_at IS NULL
    ) AS contacts_count
FROM clients c
LEFT JOIN contacts ct ON ct.id = c.primary_contact_id AND ct.deleted_at IS NULL
LEFT JOIN users am ON am.id = c.account_manager_id AND am.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status')::text)
  AND (sqlc.narg('manager_id')::uuid IS NULL OR c.account_manager_id = sqlc.narg('manager_id')::uuid)
  -- La recherche porte aussi sur le contact principal : on retrouve souvent
  -- une entreprise par le nom de la personne a qui l'on parle.
  AND (
      sqlc.narg('search')::text IS NULL
      OR c.name ILIKE '%' || sqlc.narg('search')::text || '%'
      OR (coalesce(ct.firstname, '') || ' ' || coalesce(ct.lastname, '')) ILIKE '%' || sqlc.narg('search')::text || '%'
  )
  AND (
      sqlc.narg('has_portal')::boolean IS NULL
      OR (sqlc.narg('has_portal')::boolean AND c.portal_users > 0)
      OR (NOT sqlc.narg('has_portal')::boolean AND c.portal_users = 0)
  )
ORDER BY
    CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('dir')::text = 'desc' THEN c.name END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'projects' AND sqlc.arg('dir')::text = 'asc' THEN c.projects_active END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'projects' AND sqlc.arg('dir')::text = 'desc' THEN c.projects_active END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'created' AND sqlc.arg('dir')::text = 'asc' THEN c.created_at END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'created' AND sqlc.arg('dir')::text = 'desc' THEN c.created_at END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('dir')::text = 'asc' THEN c.name END ASC,
    c.name ASC,
    -- Depart d'egalite stable : deux clients homonymes ne doivent pas changer
    -- de place d'une page a l'autre, sinon l'un des deux disparait.
    c.id ASC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountCrmClients :one
SELECT count(*)
FROM clients c
LEFT JOIN contacts ct ON ct.id = c.primary_contact_id AND ct.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR c.status = sqlc.narg('status')::text)
  AND (sqlc.narg('manager_id')::uuid IS NULL OR c.account_manager_id = sqlc.narg('manager_id')::uuid)
  AND (
      sqlc.narg('search')::text IS NULL
      OR c.name ILIKE '%' || sqlc.narg('search')::text || '%'
      OR (coalesce(ct.firstname, '') || ' ' || coalesce(ct.lastname, '')) ILIKE '%' || sqlc.narg('search')::text || '%'
  )
  AND (
      sqlc.narg('has_portal')::boolean IS NULL
      OR (sqlc.narg('has_portal')::boolean AND c.portal_users > 0)
      OR (NOT sqlc.narg('has_portal')::boolean AND c.portal_users = 0)
  );

-- name: GetCrmClient :one
-- Fiche d'un client : son en-tete, avec le contact principal et les compteurs
-- que le tableau montre deja.
SELECT
    c.*,
    ct.id                      AS contact_id,
    coalesce(ct.firstname, '') AS contact_firstname,
    coalesce(ct.lastname, '')  AS contact_lastname,
    coalesce(ct.role, '')      AS contact_role,
    ct.email                   AS contact_email,
    am.firstname               AS manager_firstname,
    am.lastname                AS manager_lastname,
    am.avatar_url              AS manager_avatar_url,
    (
        SELECT count(*) FROM contacts x
        WHERE x.client_id = c.id AND x.deleted_at IS NULL
    ) AS contacts_count
FROM clients c
LEFT JOIN contacts ct ON ct.id = c.primary_contact_id AND ct.deleted_at IS NULL
LEFT JOIN users am ON am.id = c.account_manager_id AND am.deleted_at IS NULL
WHERE c.id = $1 AND c.deleted_at IS NULL;

-- name: ListProjectsOfClient :many
-- Projets du client, tels que sa fiche les liste. Bornee : une fiche montre ce
-- qui se lit d'un coup d'oeil, pas tout l'historique d'un gros compte.
SELECT id, name, status, progress, due_on
FROM projects
WHERE client_id = sqlc.arg('client_id') AND deleted_at IS NULL
ORDER BY (status = 'livre'), due_on NULLS LAST, id
LIMIT sqlc.arg('page_size');

-- name: ListPortalUsersOfClient :many
-- Comptes de portail rattaches au client. La colonne existe, l'ecran qui la
-- remplit non : la fiche la montre pour que le jour ou on rattachera quelqu'un,
-- ce soit visible.
SELECT id, email, firstname, lastname, avatar_url, last_login_at
FROM users
WHERE client_id = sqlc.arg('client_id') AND deleted_at IS NULL
ORDER BY lastname, firstname, id
LIMIT sqlc.arg('page_size');

-- name: UpdateClient :one
-- Modification de la fiche. Les compteurs n'y sont pas — ils sont tenus par
-- declencheur — ni le contact principal, qui a sa propre route parce que la
-- cle etrangere composite impose un ordre.
UPDATE clients
SET name               = sqlc.arg('name'),
    status             = sqlc.arg('status'),
    account_manager_id = sqlc.narg('account_manager_id'),
    website            = sqlc.arg('website'),
    phone              = sqlc.arg('phone'),
    address            = sqlc.arg('address'),
    postal_code        = sqlc.arg('postal_code'),
    city               = sqlc.arg('city'),
    country            = sqlc.arg('country'),
    siret              = sqlc.arg('siret'),
    vat_number         = sqlc.arg('vat_number'),
    updated_at         = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: MoveClientStatus :one
-- Deplacement d'une carte du kanban. Distincte de UpdateClient : glisser une
-- carte ne doit pas reecrire les coordonnees de l'entreprise avec ce que
-- l'ecran avait en memoire.
UPDATE clients
SET status = sqlc.arg('status'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: ListClientsBoard :many
-- Toutes les cartes du kanban, tous statuts confondus.
--
-- Bornee et non paginee, comme le tableau des taches : un kanban se lit en
-- entier ou pas du tout. Le handler previent quand la borne est atteinte.
SELECT
    c.*,
    ct.id                      AS contact_id,
    coalesce(ct.firstname, '') AS contact_firstname,
    coalesce(ct.lastname, '')  AS contact_lastname,
    coalesce(ct.role, '')      AS contact_role,
    ct.email                   AS contact_email,
    am.firstname               AS manager_firstname,
    am.lastname                AS manager_lastname,
    am.avatar_url              AS manager_avatar_url,
    (
        SELECT count(*) FROM contacts x
        WHERE x.client_id = c.id AND x.deleted_at IS NULL
    ) AS contacts_count
FROM clients c
LEFT JOIN contacts ct ON ct.id = c.primary_contact_id AND ct.deleted_at IS NULL
LEFT JOIN users am ON am.id = c.account_manager_id AND am.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND (sqlc.narg('manager_id')::uuid IS NULL OR c.account_manager_id = sqlc.narg('manager_id')::uuid)
  AND (
      sqlc.narg('search')::text IS NULL
      OR c.name ILIKE '%' || sqlc.narg('search')::text || '%'
      OR (coalesce(ct.firstname, '') || ' ' || coalesce(ct.lastname, '')) ILIKE '%' || sqlc.narg('search')::text || '%'
  )
ORDER BY c.name, c.id
LIMIT sqlc.arg('page_size');

-- name: CountProjectsOfClient :one
-- Tous statuts confondus, livres compris : c'est ce qui decide si un client
-- peut disparaitre. Un projet livre garde la trace de qui l'a commande.
SELECT count(*) FROM projects
WHERE client_id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteClient :exec
-- Suppression logique, comme partout ailleurs : un client efface par erreur
-- doit pouvoir revenir. La designation du contact principal est retiree par la
-- meme occasion, sans quoi elle pointerait depuis une ligne morte.
UPDATE clients
SET deleted_at = now(),
    primary_contact_id = NULL,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;
