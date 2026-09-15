-- Tickets.
--
-- Les trois vues de l'ecran — tableau, kanban par projet, kanban par statut —
-- partagent la meme barre d'outils, donc les memes filtres. Ils sont ecrits une
-- fois par requete plutot que composes en Go : sqlc verifie alors le SQL a la
-- generation, ce qu'une concatenation de chaines interdirait.
--
-- `assignee_id` n'est pas un filtre mais une clause : l'ecran ne montre que les
-- tickets du compte appelant, et l'identifiant vient de la session.

-- name: ListTicketsAssignedTo :many
-- Page du tableau, du plus recemment mis a jour au plus ancien — l'ordre dans
-- lequel on reprend son travail.
--
-- La recherche porte sur le sujet et sur le numero : « 47 » doit retrouver le
-- ticket #47, c'est ainsi qu'on le designe a l'oral.
SELECT
    t.id,
    t.numero,
    t.subject,
    t.tracker,
    t.status,
    t.priority,
    t.created_at,
    t.updated_at,
    p.id   AS project_id,
    p.name AS project_name
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg('assignee_id')
  AND (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (sqlc.narg('tracker')::text IS NULL OR t.tracker = sqlc.narg('tracker')::text)
  AND (sqlc.narg('priority')::text IS NULL OR t.priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR t.project_id = sqlc.narg('project_id')::uuid)
  AND (
      sqlc.narg('search')::text IS NULL
      OR t.subject ILIKE '%' || sqlc.narg('search')::text || '%'
      OR t.numero::text = sqlc.narg('search')::text
  )
ORDER BY t.updated_at DESC, t.numero DESC
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountTicketsAssignedTo :one
-- Total pour la pagination, aux memes conditions que la liste.
SELECT count(*)
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg('assignee_id')
  AND (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (sqlc.narg('tracker')::text IS NULL OR t.tracker = sqlc.narg('tracker')::text)
  AND (sqlc.narg('priority')::text IS NULL OR t.priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR t.project_id = sqlc.narg('project_id')::uuid)
  AND (
      sqlc.narg('search')::text IS NULL
      OR t.subject ILIKE '%' || sqlc.narg('search')::text || '%'
      OR t.numero::text = sqlc.narg('search')::text
  );

-- name: ListTicketsBoardAssignedTo :many
-- Toutes les cartes des deux kanbans, aux memes filtres que le tableau.
--
-- Bornee et non paginee, comme les kanbans des taches et des clients : un
-- tableau se lit en entier ou pas du tout, et paginer une colonne n'aurait
-- aucun sens. Le handler previent quand la borne a coupe.
--
-- Trie par numero decroissant et non par mise a jour : dans un tableau, c'est
-- la colonne qui porte le classement, et l'ordre a l'interieur doit rester
-- stable d'un affichage a l'autre.
SELECT
    t.id,
    t.numero,
    t.subject,
    t.tracker,
    t.status,
    t.priority,
    t.created_at,
    t.updated_at,
    p.id   AS project_id,
    p.name AS project_name
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND t.assignee_id = sqlc.arg('assignee_id')
  AND (sqlc.narg('status')::text IS NULL OR t.status = sqlc.narg('status')::text)
  AND (sqlc.narg('tracker')::text IS NULL OR t.tracker = sqlc.narg('tracker')::text)
  AND (sqlc.narg('priority')::text IS NULL OR t.priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR t.project_id = sqlc.narg('project_id')::uuid)
  AND (
      sqlc.narg('search')::text IS NULL
      OR t.subject ILIKE '%' || sqlc.narg('search')::text || '%'
      OR t.numero::text = sqlc.narg('search')::text
  )
ORDER BY t.numero DESC
LIMIT sqlc.arg('page_size');

-- name: CreateTicket :one
-- Depot d'un ticket.
--
-- Le numero n'est pas fourni : l'identite de la colonne l'attribue, ce qui
-- rend deux creations simultanees incapables d'obtenir le meme.
--
-- La CTE evite un second aller-retour pour le nom du projet : la ligne rendue a
-- la meme forme que celles de la liste, et l'ecran peut l'afficher telle quelle.
WITH nouveau AS (
    INSERT INTO tickets (
        project_id, subject, description, tracker, status, priority,
        assignee_id, created_by
    )
    VALUES (
        sqlc.arg('project_id'), sqlc.arg('subject'), sqlc.arg('description'),
        sqlc.arg('tracker'), sqlc.arg('status'), sqlc.arg('priority'),
        sqlc.narg('assignee_id'), sqlc.narg('created_by')
    )
    RETURNING *
)
SELECT
    n.id,
    n.numero,
    n.subject,
    n.tracker,
    n.status,
    n.priority,
    n.created_at,
    n.updated_at,
    p.id   AS project_id,
    p.name AS project_name
FROM nouveau n
JOIN projects p ON p.id = n.project_id;

-- name: GetTicket :one
-- Fiche d'un ticket : son en-tete et ses coordonnees.
--
-- Les messages et les evenements font l'objet de deux requetes a part : les
-- ramener par jointure multiplierait l'en-tete par le nombre de lignes du
-- registre.
SELECT
    t.id,
    t.numero,
    t.subject,
    t.description,
    t.tracker,
    t.status,
    t.priority,
    t.created_at,
    t.updated_at,
    p.id   AS project_id,
    p.name AS project_name,
    c.id   AS client_id,
    c.name AS client_name,
    t.assignee_id,
    a.firstname  AS assignee_firstname,
    a.lastname   AS assignee_lastname,
    a.avatar_url AS assignee_avatar_url,
    t.created_by,
    r.firstname  AS reporter_firstname,
    r.lastname   AS reporter_lastname,
    r.avatar_url AS reporter_avatar_url
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
LEFT JOIN clients c ON c.id = p.client_id AND c.deleted_at IS NULL
LEFT JOIN users a ON a.id = t.assignee_id AND a.deleted_at IS NULL
LEFT JOIN users r ON r.id = t.created_by AND r.deleted_at IS NULL
WHERE t.id = sqlc.arg('id') AND t.deleted_at IS NULL;

-- name: ListTicketMessages :many
-- Ce qui s'est dit sur un ticket, du plus ancien au plus recent.
SELECT
    m.id,
    m.body,
    m.is_internal,
    m.created_at,
    m.author_id,
    u.firstname  AS author_firstname,
    u.lastname   AS author_lastname,
    u.avatar_url AS author_avatar_url,
    u.role       AS author_role
FROM ticket_messages m
LEFT JOIN users u ON u.id = m.author_id AND u.deleted_at IS NULL
WHERE m.ticket_id = sqlc.arg('ticket_id') AND m.deleted_at IS NULL
ORDER BY m.created_at, m.id;

-- name: ListTicketEvents :many
-- Ce qui est arrive au ticket, dans le meme ordre.
SELECT
    e.id,
    e.field,
    e.old_value,
    e.new_value,
    e.created_at,
    e.actor_id,
    u.firstname  AS actor_firstname,
    u.lastname   AS actor_lastname,
    u.avatar_url AS actor_avatar_url
FROM ticket_events e
LEFT JOIN users u ON u.id = e.actor_id AND u.deleted_at IS NULL
WHERE e.ticket_id = sqlc.arg('ticket_id')
ORDER BY e.created_at, e.id;

-- name: CreateTicketMessage :one
-- Inscrit un message au registre.
INSERT INTO ticket_messages (ticket_id, author_id, body, is_internal)
VALUES (sqlc.arg('ticket_id'), sqlc.narg('author_id'), sqlc.arg('body'), sqlc.arg('is_internal'))
RETURNING id, created_at;

-- name: CreateTicketEvent :one
-- Inscrit un changement au journal.
INSERT INTO ticket_events (ticket_id, actor_id, field, old_value, new_value)
VALUES (
    sqlc.arg('ticket_id'), sqlc.narg('actor_id'),
    sqlc.arg('field'), sqlc.arg('old_value'), sqlc.arg('new_value')
)
RETURNING id, created_at;

-- name: UpdateTicketFields :exec
-- Applique les changements qu'une entree du registre porte.
--
-- Un `coalesce` par champ : ce qui n'est pas demande garde sa valeur, ce qui
-- evite de relire puis reecrire les deux autres colonnes a chaque fois.
--
-- L'assigne a son propre drapeau parce que NULL y est une valeur qui veut dire
-- quelque chose — « remettre a prendre ». Sans lui, on ne saurait pas
-- distinguer « ne touche pas a l'assignation » de « retire-la ».
--
-- Ne rend rien : le service relit la fiche dans la meme transaction pour
-- comparer l'avant et l'apres, et en tirer les lignes du journal.
UPDATE tickets
SET status      = coalesce(sqlc.narg('status')::text, status),
    priority    = coalesce(sqlc.narg('priority')::text, priority),
    assignee_id = CASE
                      WHEN sqlc.arg('change_assignee')::boolean
                          THEN sqlc.narg('assignee_id')::uuid
                      ELSE assignee_id
                  END,
    updated_at  = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: UpdateTicketSubject :exec
-- Renomme un ticket. Le numero, lui, ne bouge jamais.
UPDATE tickets
SET subject = sqlc.arg('subject'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: LockTicket :one
-- Prend le verrou d'ecriture sur un ticket, le temps de la transaction.
--
-- Sans lui, deux ecritures concurrentes lisent toutes deux l'etat d'avant et
-- inscrivent chacune sa ligne au journal : on se retrouve avec deux
-- « renomme de A vers B » pour un seul renommage. Le verrou les met en file,
-- la seconde voit le travail de la premiere et n'a plus rien a journaliser.
SELECT id
FROM tickets
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
FOR UPDATE;
