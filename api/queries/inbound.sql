-- E-mail entrant : reglages, file des e-mails recus, tri.

-- name: GetInboundSettings :one
SELECT * FROM inbound_mail_settings WHERE id = 1;

-- name: UpdateInboundSettings :exec
-- Le mot de passe n'est remplace que s'il est fourni : l'ecran ne le relit
-- jamais, et un champ laisse vide garde celui en place.
UPDATE inbound_mail_settings SET
    address           = sqlc.arg('address'),
    imap_enabled      = sqlc.arg('imap_enabled'),
    imap_host         = sqlc.arg('imap_host'),
    imap_port         = sqlc.arg('imap_port'),
    imap_security     = sqlc.arg('imap_security'),
    imap_username     = sqlc.arg('imap_username'),
    imap_folder       = sqlc.arg('imap_folder'),
    imap_password_enc = CASE WHEN sqlc.arg('set_password')::boolean THEN sqlc.narg('imap_password_enc') ELSE imap_password_enc END,
    updated_at        = now()
WHERE id = 1;

-- name: SetInboundWebhookSecret :exec
UPDATE inbound_mail_settings SET webhook_secret = sqlc.arg('secret'), updated_at = now() WHERE id = 1;

-- name: RequestInboundPoll :exec
UPDATE inbound_mail_settings SET poll_requested_at = now() WHERE id = 1;

-- name: RecordInboundPoll :exec
UPDATE inbound_mail_settings
SET last_poll_at = now(), last_poll_error = sqlc.arg('error'), poll_requested_at = NULL
WHERE id = 1;

-- name: InsertInboundEmail :many
-- Un e-mail deja recu — releve rejouee, webhook renvoye — ne rentre pas une
-- seconde fois : la ligne n'est pas rendue.
INSERT INTO inbound_emails (source, message_id, from_address, from_name, subject, excerpt, attachment_count, raw)
VALUES (
    sqlc.arg('source'), sqlc.arg('message_id'), sqlc.arg('from_address'), sqlc.arg('from_name'),
    sqlc.arg('subject'), sqlc.arg('excerpt'), sqlc.arg('attachment_count'), sqlc.arg('raw')
)
ON CONFLICT (message_id) WHERE message_id <> '' DO NOTHING
RETURNING id;

-- name: ClaimPendingInbound :one
-- Le prochain e-mail a ranger, verrouille le temps de le traiter : deux
-- instances pendant un deploiement ne rangent pas deux fois le meme.
SELECT * FROM inbound_emails
WHERE status = 'pending'
ORDER BY received_at
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: LockInboundEmail :one
SELECT * FROM inbound_emails WHERE id = sqlc.arg('id') FOR UPDATE;

-- name: FinishInboundEmail :exec
UPDATE inbound_emails SET
    status       = sqlc.arg('status'),
    reason       = sqlc.arg('reason'),
    client_id    = sqlc.narg('client_id'),
    ticket_id    = sqlc.narg('ticket_id'),
    error        = sqlc.arg('error'),
    processed_at = now()
WHERE id = sqlc.arg('id');

-- name: FailInboundAttempt :exec
-- Une erreur passagere (base, disque) : on reessaie, cinq fois au plus.
UPDATE inbound_emails SET
    attempts = attempts + 1,
    error    = sqlc.arg('error'),
    status   = CASE WHEN attempts + 1 >= 5 THEN 'failed' ELSE 'pending' END
WHERE id = sqlc.arg('id');

-- name: ListHeldInbound :many
-- Les e-mails a trier, les plus recents d'abord.
SELECT e.id, e.received_at, e.source, e.from_address, e.from_name, e.subject, e.excerpt,
       e.attachment_count, e.reason, e.client_id, c.name AS client_name
FROM inbound_emails e
LEFT JOIN clients c ON c.id = e.client_id AND c.deleted_at IS NULL
WHERE e.status = 'held'
ORDER BY e.received_at DESC, e.id
LIMIT sqlc.arg('max_rows') OFFSET sqlc.arg('skip');

-- name: CountHeldInbound :one
SELECT count(*)::integer FROM inbound_emails WHERE status = 'held';

-- name: GetInboundEmail :one
SELECT e.*, c.name AS client_name
FROM inbound_emails e
LEFT JOIN clients c ON c.id = e.client_id AND c.deleted_at IS NULL
WHERE e.id = sqlc.arg('id');

-- name: InboundStats :one
SELECT
    count(*) FILTER (WHERE status = 'pending')::integer AS pending,
    count(*) FILTER (WHERE status = 'held')::integer    AS held,
    count(*) FILTER (WHERE status = 'processed' AND processed_at > now() - interval '24 hours')::integer AS processed_day,
    count(*) FILTER (WHERE status = 'failed' AND received_at > now() - interval '7 days')::integer AS failed_week
FROM inbound_emails
WHERE status IN ('pending', 'held') OR received_at > now() - interval '7 days';

-- name: CountTicketsOpenedByEmailFrom :one
-- Les tickets qu'une adresse a ouverts par e-mail dans l'heure : au-dela d'un
-- seuil, deux robots se repondent, et l'e-mail attend un humain.
SELECT count(*)::integer FROM inbound_emails
WHERE from_address = sqlc.arg('from_address')
  AND status = 'processed' AND reason = 'created'
  AND received_at > now() - interval '1 hour';

-- name: FindActiveUserByEmail :one
SELECT id, role, client_id, firstname, lastname
FROM users
WHERE email = sqlc.arg('email') AND deleted_at IS NULL AND disabled_at IS NULL;

-- name: FindContactsByEmail :many
-- Les contacts du CRM qui portent cette adresse, sur des clients actifs.
SELECT ct.id, ct.client_id, ct.firstname, ct.lastname
FROM contacts ct
JOIN clients c ON c.id = ct.client_id AND c.deleted_at IS NULL
WHERE ct.email = sqlc.arg('email') AND ct.deleted_at IS NULL
LIMIT 10;

-- name: ListTicketProjectsOfClient :many
-- Les projets d'un client qui recoivent des tickets : en cours, en attente,
-- ou heberges. Un projet livre et clos n'en attend plus.
SELECT p.id, p.name, p.status
FROM projects p
WHERE p.client_id = sqlc.arg('client_id')
  AND p.deleted_at IS NULL
  AND NOT p.is_internal
  AND p.status IN ('cadrage', 'production', 'attente', 'hebergement')
ORDER BY p.updated_at DESC, p.id
LIMIT 20;

-- name: ListTicketProjectsOfClients :many
-- Les memes, pour plusieurs clients d'un coup : la liste a trier propose a
-- chaque e-mail les projets de son client.
SELECT p.client_id, p.id, p.name
FROM projects p
WHERE p.client_id = ANY(sqlc.arg('client_ids')::uuid[])
  AND p.deleted_at IS NULL
  AND NOT p.is_internal
  AND p.status IN ('cadrage', 'production', 'attente', 'hebergement')
ORDER BY p.updated_at DESC, p.id
LIMIT 500;

-- name: ListTriageRecipients :many
-- Qui prevenir d'un e-mail a trier : les administrateurs actifs.
SELECT id FROM users
WHERE role = 'admin' AND deleted_at IS NULL AND disabled_at IS NULL
LIMIT 50;

-- name: GetProjectClientForTriage :one
-- Le client d'un projet choisi au tri. Un projet interne n'en a pas a qui
-- repondre : il est refuse.
SELECT client_id FROM projects
WHERE id = sqlc.arg('id') AND deleted_at IS NULL AND NOT is_internal;
