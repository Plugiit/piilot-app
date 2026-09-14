-- name: ListCrmContacts :many
-- Liste paginee de l'ecran « Contacts ».
--
-- Jointure a gauche : un contact libre n'a pas encore d'entreprise, et doit
-- rester visible — c'est meme la seule liste ou on le retrouve.
--
-- `is_primary` dit si la personne est l'interlocuteur principal de son client.
-- `coalesce` et non la seule egalite : chez un client qui n'a designe
-- personne, `primary_contact_id` est NULL et la comparaison rendrait NULL —
-- que le scan Go refuserait dans un booleen. Le cast explicite est la pour
-- sqlc, qui sans lui rend un `interface{}`.
SELECT
    ct.*,
    c.name AS client_name,
    coalesce(c.primary_contact_id = ct.id, false)::boolean AS is_primary
FROM contacts ct
LEFT JOIN clients c ON c.id = ct.client_id AND c.deleted_at IS NULL
WHERE ct.deleted_at IS NULL
  AND (sqlc.narg('client_id')::uuid IS NULL OR ct.client_id = sqlc.narg('client_id')::uuid)
  AND (NOT sqlc.arg('only_free')::boolean OR ct.client_id IS NULL)
  AND (
      sqlc.narg('search')::text IS NULL
      OR (ct.firstname || ' ' || ct.lastname) ILIKE '%' || sqlc.narg('search')::text || '%'
      OR ct.email ILIKE '%' || sqlc.narg('search')::text || '%'
      OR c.name ILIKE '%' || sqlc.narg('search')::text || '%'
  )
ORDER BY
    CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('dir')::text = 'desc'
         THEN ct.lastname || ' ' || ct.firstname END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'client' AND sqlc.arg('dir')::text = 'asc' THEN c.name END ASC,
    CASE WHEN sqlc.arg('sort')::text = 'client' AND sqlc.arg('dir')::text = 'desc' THEN c.name END DESC,
    CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('dir')::text = 'asc'
         THEN ct.lastname || ' ' || ct.firstname END ASC,
    ct.lastname ASC,
    ct.firstname ASC,
    -- Depart d'egalite stable : deux homonymes ne doivent pas changer de place
    -- d'une page a l'autre, sinon l'un des deux disparait.
    ct.id ASC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountCrmContacts :one
SELECT count(*)
FROM contacts ct
LEFT JOIN clients c ON c.id = ct.client_id AND c.deleted_at IS NULL
WHERE ct.deleted_at IS NULL
  AND (sqlc.narg('client_id')::uuid IS NULL OR ct.client_id = sqlc.narg('client_id')::uuid)
  AND (NOT sqlc.arg('only_free')::boolean OR ct.client_id IS NULL)
  AND (
      sqlc.narg('search')::text IS NULL
      OR (ct.firstname || ' ' || ct.lastname) ILIKE '%' || sqlc.narg('search')::text || '%'
      OR ct.email ILIKE '%' || sqlc.narg('search')::text || '%'
      OR c.name ILIKE '%' || sqlc.narg('search')::text || '%'
  );

-- name: CreateContact :one
-- `client_id` peut etre nul : le contact est alors libre, en attente d'une
-- entreprise.
INSERT INTO contacts (client_id, firstname, lastname, role, email, phone)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetContact :one
SELECT ct.*, c.name AS client_name
FROM contacts ct
LEFT JOIN clients c ON c.id = ct.client_id AND c.deleted_at IS NULL
WHERE ct.id = $1 AND ct.deleted_at IS NULL;

-- name: ListContactsOfClient :many
-- Alimente le menu deroulant qui designe le contact principal d'un client.
--
-- Il propose deux ensembles : les contacts du client, et les contacts libres —
-- ces derniers etant rattaches au moment ou on les choisit. Sans eux, un
-- contact cree sans entreprise n'aurait aucun moyen d'en rejoindre une.
--
-- Les contacts d'un AUTRE client restent exclus : la cle etrangere composite
-- les refuserait, et les proposer laisserait croire qu'on peut se les prendre.
-- Cast explicite pour sqlc, qui sans lui rend un `interface{}`.
SELECT *, (client_id IS NULL)::boolean AS is_free
FROM contacts
WHERE deleted_at IS NULL
  AND (client_id = sqlc.arg('client_id') OR client_id IS NULL)
  AND (
      sqlc.narg('search')::text IS NULL
      OR (firstname || ' ' || lastname) ILIKE '%' || sqlc.narg('search')::text || '%'
  )
-- Les siens d'abord : ce sont eux qu'on cherche le plus souvent.
ORDER BY (client_id IS NULL), lastname, firstname, id
LIMIT sqlc.arg('page_size');

-- name: ListFreeContacts :many
-- Contacts sans entreprise, tels que le formulaire de creation d'un client les
-- propose. Un contact deja rattache n'y figure pas : il appartient a un autre
-- client, et le nouveau ne peut pas le lui prendre.
SELECT * FROM contacts
WHERE client_id IS NULL
  AND deleted_at IS NULL
  AND (
      sqlc.narg('search')::text IS NULL
      OR (firstname || ' ' || lastname) ILIKE '%' || sqlc.narg('search')::text || '%'
  )
ORDER BY lastname, firstname, id
LIMIT sqlc.arg('page_size');

-- name: AttachContactToClient :one
-- Rattache un contact libre a un client.
--
-- La condition sur `client_id IS NULL` fait de cette requete une prise de
-- possession sure : deux clients crees en meme temps avec le meme contact, un
-- seul l'obtient, et l'autre ne touche aucune ligne.
UPDATE contacts
SET client_id = sqlc.arg('client_id'),
    updated_at = now()
WHERE id = sqlc.arg('contact_id')
  AND client_id IS NULL
  AND deleted_at IS NULL
RETURNING *;

-- name: SetPrimaryContact :exec
-- Designe le contact principal. `NULL` le retire.
--
-- La condition sur `client_id` est une ceinture : la cle etrangere composite
-- refuse deja le contact d'un autre client, mais elle rendrait une erreur de
-- contrainte la ou un zero ligne touchee se traduit en « introuvable ».
UPDATE clients
SET primary_contact_id = sqlc.narg('contact_id'),
    updated_at = now()
WHERE clients.id = sqlc.arg('client_id')
  AND clients.deleted_at IS NULL
  AND (
      sqlc.narg('contact_id')::uuid IS NULL
      OR EXISTS (
          SELECT 1 FROM contacts ct
          WHERE ct.id = sqlc.narg('contact_id')::uuid
            AND ct.client_id = sqlc.arg('client_id')
            AND ct.deleted_at IS NULL
      )
  );

-- name: UpdateContact :one
-- Modification de l'identite. Le rattachement a un client n'est pas ici : il
-- passe par la designation du contact principal, qui sait tenir la cle
-- etrangere composite dans le bon ordre.
UPDATE contacts
SET firstname = sqlc.arg('firstname'),
    lastname  = sqlc.arg('lastname'),
    role      = sqlc.arg('role'),
    email     = sqlc.narg('email'),
    phone     = sqlc.arg('phone'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: ClearPrimaryContactOf :exec
-- Retire la designation qui pointe vers ce contact, quel que soit le client.
-- Appelee avant la suppression logique : la cle etrangere ne se declenche que
-- sur un DELETE reel, et laisserait sinon un client designant un contact mort.
UPDATE clients
SET primary_contact_id = NULL,
    updated_at = now()
WHERE primary_contact_id = $1;

-- name: SoftDeleteContact :exec
UPDATE contacts
SET deleted_at = now(),
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteContactsOfClient :exec
-- Les contacts d'un client efface le sont avec lui : ils n'existaient que pour
-- lui. La suppression est logique, comme celle du client, et se defait donc de
-- la meme facon.
UPDATE contacts
SET deleted_at = now(),
    updated_at = now()
WHERE client_id = $1 AND deleted_at IS NULL;
