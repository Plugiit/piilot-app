-- Portail client : tickets.
--
-- Meme regle que le reste du portail : chaque requete de lecture part de
-- l'appelant et de son client. Un ticket interne a l'agence, ou celui d'un
-- autre client, ne rend aucune ligne.
--
-- Les notes internes ne sortent jamais d'ici : la requete des messages les
-- ecarte elle-meme, plutot que de compter sur le code qui l'appelle.

-- name: PortalListTickets :many
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
FROM users u
JOIN projects p ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
JOIN tickets t  ON t.project_id = p.id AND t.client_visible AND t.deleted_at IS NULL
WHERE u.id = sqlc.arg('user_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL
  AND (sqlc.narg('open')::boolean IS NULL
       OR (t.status NOT IN ('done', 'annule')) = sqlc.narg('open')::boolean)
ORDER BY t.updated_at DESC, t.id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: PortalCountTickets :one
SELECT count(*)
FROM users u
JOIN projects p ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
JOIN tickets t  ON t.project_id = p.id AND t.client_visible AND t.deleted_at IS NULL
WHERE u.id = sqlc.arg('user_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL
  AND (sqlc.narg('open')::boolean IS NULL
       OR (t.status NOT IN ('done', 'annule')) = sqlc.narg('open')::boolean);

-- name: PortalGetTicket :one
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
    r.firstname AS reporter_firstname
FROM users u
JOIN projects p ON p.client_id = u.client_id AND p.deleted_at IS NULL AND NOT p.is_internal
JOIN tickets t  ON t.project_id = p.id AND t.client_visible AND t.deleted_at IS NULL
LEFT JOIN users r ON r.id = t.created_by AND r.deleted_at IS NULL
WHERE u.id = sqlc.arg('user_id')
  AND t.id = sqlc.arg('ticket_id')
  AND u.role = 'client'
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL;

-- name: PortalListTicketMessages :many
-- Les messages publics d'un ticket. Les notes internes n'en sortent pas.
SELECT
    m.id,
    m.body,
    m.created_at,
    u.firstname AS author_firstname,
    u.role      AS author_role,
    m.via_email,
    m.sender_name
FROM ticket_messages m
LEFT JOIN users u ON u.id = m.author_id AND u.deleted_at IS NULL
WHERE m.ticket_id = sqlc.arg('ticket_id')
  AND NOT m.is_internal
  AND m.deleted_at IS NULL
ORDER BY m.created_at, m.id
LIMIT 500;

-- name: PortalListTicketStatusEvents :many
-- Les changements de statut, seuls : qui traite le ticket ou sa priorite
-- interne ne regardent pas le client.
SELECT e.id, e.old_value, e.new_value, e.created_at
FROM ticket_events e
WHERE e.ticket_id = sqlc.arg('ticket_id') AND e.field = 'status'
ORDER BY e.created_at, e.id
LIMIT 200;

-- name: ListTicketFiles :many
SELECT * FROM attachments
WHERE ticket_id = sqlc.arg('ticket_id')
ORDER BY created_at
LIMIT 50;

-- name: CreateTicketFile :one
INSERT INTO attachments (ticket_id, filename, content_type, size_bytes, storage_key, uploaded_by)
VALUES (sqlc.arg('ticket_id'), sqlc.arg('filename'), sqlc.arg('content_type'),
        sqlc.arg('size_bytes'), sqlc.arg('storage_key'), sqlc.narg('uploaded_by'))
RETURNING *;

-- name: CountTicketFiles :one
SELECT count(*) FROM attachments WHERE ticket_id = $1;

-- name: GetTicketMailContext :one
-- De quoi ecrire les e-mails d'un ticket : son numero, son sujet, son projet,
-- et la personne du portail qui l'a ouvert, si elle est toujours active.
SELECT
    t.numero,
    t.subject,
    t.client_visible,
    p.name AS project_name,
    c.name AS client_name,
    r.email     AS reporter_email,
    r.firstname AS reporter_firstname,
    coalesce(r.role = 'client' AND r.deleted_at IS NULL AND r.disabled_at IS NULL, false)::boolean AS reporter_is_active_client,
    -- Sans compte du portail : la personne qui a ecrit a l'adresse de support.
    t.requester_email,
    t.requester_name
FROM tickets t
JOIN projects p ON p.id = t.project_id
JOIN clients c  ON c.id = p.client_id
LEFT JOIN users r ON r.id = t.created_by
WHERE t.id = sqlc.arg('ticket_id');

-- name: ListTicketTeamMailRecipients :many
-- Qui prevenir par e-mail cote agence : la personne qui traite le ticket, ou
-- les administrateurs tant que personne ne l'a pris. Le role dit vers quel
-- domaine pointer le lien, quand chaque espace a le sien.
SELECT u.email, u.firstname, u.role
FROM tickets t
JOIN users u ON (
    (t.assignee_id IS NOT NULL AND u.id = t.assignee_id)
    OR (t.assignee_id IS NULL AND u.role = 'admin')
)
WHERE t.id = sqlc.arg('ticket_id')
  AND u.deleted_at IS NULL
  AND u.disabled_at IS NULL
LIMIT 20;

-- name: SetTicketClientVisible :exec
UPDATE tickets SET client_visible = true WHERE id = $1;
