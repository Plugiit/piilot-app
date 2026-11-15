-- Effacement RGPD (1.5) : demandes avec delai, puis execution en une
-- transaction. Les requetes de suppression rendent leur nombre de lignes : il
-- alimente le resume de l'effacement, qui ne contient ni nom ni adresse.

-- name: CreateRgpdErasure :one
INSERT INTO rgpd_erasures (subject_type, subject_id, subject_label, reason, scheduled_for, requested_by, prev_disabled)
VALUES (
    sqlc.arg('subject_type'), sqlc.arg('subject_id'), sqlc.arg('subject_label'), sqlc.arg('reason'),
    sqlc.arg('scheduled_for'), sqlc.narg('requested_by'), sqlc.narg('prev_disabled')
)
RETURNING *;

-- name: GetRgpdErasureForUpdate :one
SELECT * FROM rgpd_erasures WHERE id = $1 FOR UPDATE;

-- name: ListRgpdErasures :many
SELECT * FROM rgpd_erasures
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('subject_type')::text IS NULL OR subject_type = sqlc.narg('subject_type')::text)
  AND (sqlc.narg('subject_id')::uuid IS NULL OR subject_id = sqlc.narg('subject_id')::uuid)
ORDER BY created_at DESC, id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountRgpdErasures :one
SELECT count(*) FROM rgpd_erasures
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('subject_type')::text IS NULL OR subject_type = sqlc.narg('subject_type')::text)
  AND (sqlc.narg('subject_id')::uuid IS NULL OR subject_id = sqlc.narg('subject_id')::uuid);

-- name: ClaimDueRgpdErasure :one
-- Reserve le prochain effacement arrive a echeance, sous verrou, pour la
-- duree de la transaction de l'appelant : une autre instance passe au suivant.
SELECT * FROM rgpd_erasures
WHERE status = 'scheduled' AND scheduled_for <= now()
ORDER BY scheduled_for, id
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: CancelRgpdErasure :exec
UPDATE rgpd_erasures
SET status = 'cancelled', cancelled_by = sqlc.narg('cancelled_by'), cancelled_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'scheduled';

-- name: CancelInactivityErasureOfUser :one
-- Une connexion reussie annule l'effacement planifie pour inactivite.
UPDATE rgpd_erasures
SET status = 'cancelled', cancelled_by = subject_id, cancelled_at = now(), updated_at = now()
WHERE subject_type = 'user' AND subject_id = $1 AND reason = 'inactivity' AND status = 'scheduled'
RETURNING id;

-- name: ListOtherScheduledErasuresOfUser :many
-- Les autres demandes d'effacement planifiees qui visent ce compte, soit
-- directement, soit par son client. Seules les demandes (pas la purge des
-- inactifs) desactivent le compte.
SELECT e.* FROM rgpd_erasures e
WHERE e.status = 'scheduled' AND e.reason = 'request' AND e.id <> sqlc.arg('excluded')::uuid
  AND (
      (e.subject_type = 'user' AND e.subject_id = sqlc.arg('user_id')::uuid)
      OR (e.subject_type = 'client' AND e.subject_id = (SELECT u.client_id FROM users u WHERE u.id = sqlc.arg('user_id')::uuid))
  )
ORDER BY e.created_at, e.id
FOR UPDATE;

-- name: SetRgpdErasurePrevDisabled :exec
UPDATE rgpd_erasures SET prev_disabled = $2, updated_at = now() WHERE id = $1;

-- name: MarkRgpdErasureDone :exec
UPDATE rgpd_erasures
SET status = 'done', executed_at = now(), summary = sqlc.arg('summary'), subject_label = 'Supprimé',
    prev_disabled = NULL, error = '', updated_at = now()
WHERE id = sqlc.arg('id');

-- name: MarkRgpdErasureFailed :exec
UPDATE rgpd_erasures
SET status = 'failed', error = sqlc.arg('error'), updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'scheduled';

-- name: ListInactiveUsers :many
-- Comptes non administrateurs sans connexion depuis `cutoff`, qui n'ont pas
-- deja un effacement en cours.
SELECT u.id, u.email::text AS email, u.firstname, btrim(u.firstname || ' ' || u.lastname) AS label
FROM users u
WHERE u.deleted_at IS NULL AND u.role NOT IN ('admin', 'bot')
  AND coalesce(u.last_login_at, u.created_at) < sqlc.arg('cutoff')::timestamptz
  AND NOT EXISTS (
      SELECT 1 FROM rgpd_erasures e
      WHERE e.subject_type = 'user' AND e.subject_id = u.id AND e.status IN ('scheduled', 'running')
  )
ORDER BY u.id
LIMIT sqlc.arg('batch');

-- name: LockUserForErasure :one
SELECT id, email::text AS email, role, client_id, avatar_url, disabled_at, deleted_at
FROM users WHERE id = $1 FOR UPDATE;

-- name: LockClientForErasure :one
-- Sans filtre sur deleted_at : un client supprime logiquement dans le CRM garde
-- ses donnees, et doit pouvoir etre efface (demande comme execution).
SELECT id, name FROM clients WHERE id = $1 FOR UPDATE;

-- name: ListClientUsersForErasure :many
-- Les comptes du portail d'un client, effaces ou non (la cle etrangere
-- RESTRICT les compte tous).
SELECT id, email::text AS email, avatar_url, disabled_at, deleted_at
FROM users WHERE client_id = $1
ORDER BY id
FOR UPDATE;

-- name: SetUserDisabledAt :exec
UPDATE users SET disabled_at = sqlc.narg('disabled_at'), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: DisableUserKeepingDate :exec
UPDATE users SET disabled_at = coalesce(disabled_at, now()), updated_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: EraseDeleteRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE user_id = $1;

-- name: EraseDeleteOAuthGrants :execrows
-- Les codes et les jetons suivent par cascade.
DELETE FROM oauth_grants WHERE user_id = $1;

-- name: EraseDeleteNotifications :execrows
DELETE FROM notifications WHERE user_id = $1;

-- name: EraseDeleteTimers :execrows
DELETE FROM time_timers WHERE user_id = $1;

-- name: EraseDeleteFavorites :execrows
DELETE FROM project_favorites WHERE user_id = $1;

-- name: EraseDeletePasswordResets :execrows
DELETE FROM password_resets WHERE user_id = $1;

-- name: EraseDeleteInvitations :execrows
DELETE FROM invitations WHERE user_id = sqlc.arg('user_id') OR email = sqlc.arg('email')::citext;

-- name: EraseDeleteMemberships :execrows
DELETE FROM project_members WHERE user_id = $1;

-- name: EraseDeleteAssignments :execrows
DELETE FROM task_assignees WHERE user_id = $1;

-- name: EraseDeleteReportOptOuts :execrows
DELETE FROM project_report_opt_outs WHERE user_id = $1;

-- name: EraseDeleteOutboxTo :execrows
DELETE FROM email_outbox WHERE lower(to_address) = lower(sqlc.arg('email')::text);

-- name: EraseDeleteReportRecipient :execrows
DELETE FROM project_report_recipients WHERE email = sqlc.arg('email')::citext;

-- name: EraseScrubReportRecipients :execrows
UPDATE reports SET recipients = array_remove(recipients, sqlc.arg('email')::text)
WHERE sqlc.arg('email')::text = ANY(recipients);

-- name: EraseBlankTicketRequester :execrows
UPDATE tickets SET requester_email = NULL, requester_name = ''
WHERE requester_email = sqlc.arg('email')::citext;

-- name: EraseBlankMessageSender :execrows
UPDATE ticket_messages SET sender_name = '', sender_email = ''
WHERE lower(sender_email) = lower(sqlc.arg('email')::text);

-- name: EraseDeleteImportTracesOfUser :one
-- Lignes de rapport d'import dont le libelle contient l'adresse du compte (le
-- libelle d'une saisie de temps sans projet est l'e-mail) et cles d'import qui
-- le designent. La comparaison est une recherche de sous-chaine, pas un LIKE :
-- une adresse contient des `_` et des `%` possibles.
WITH rows_gone AS (
    DELETE FROM import_rows
    WHERE entity_id = sqlc.arg('user_id')::uuid
       OR position(lower(sqlc.arg('email')::text) in lower(label)) > 0
    RETURNING 1
), keys_gone AS (
    DELETE FROM import_keys WHERE entity_id = sqlc.arg('user_id')::uuid RETURNING 1
)
SELECT ((SELECT count(*) FROM rows_gone) + (SELECT count(*) FROM keys_gone))::bigint AS erased;

-- name: EraseDeleteInboundFrom :execrows
DELETE FROM inbound_emails WHERE from_address = sqlc.arg('email')::citext;

-- name: EraseAnonymizeUser :exec
-- Adresse inutilisable, champs personnels vides, mot de passe qui ne
-- correspond a aucun hachage, double authentification retiree. La ligne reste
-- pour que le temps saisi (cascade de time_entries) soit conserve.
UPDATE users SET
    email = sqlc.arg('email')::citext, password_hash = '!', firstname = '', lastname = '', gender = '',
    phone = '', address = '', postal_code = '', city = '', country = '', booking_url = '',
    avatar_url = NULL, totp_secret = NULL, last_login_at = NULL,
    client_id = CASE WHEN sqlc.arg('detach')::boolean THEN NULL ELSE client_id END,
    disabled_at = coalesce(disabled_at, now()), deleted_at = coalesce(deleted_at, now()), updated_at = now()
WHERE id = sqlc.arg('id');

-- name: EraseExportKeysOfSubject :many
-- Les archives d'export du sujet contiennent ses donnees : leurs fichiers
-- sont a supprimer.
SELECT file_key FROM rgpd_exports
WHERE subject_type = $1 AND subject_id = $2 AND file_key <> '';

-- name: EraseRedactExportsOfSubject :execrows
-- Ligne d'historique conservee, sans libelle ni fichier.
UPDATE rgpd_exports
SET status = CASE WHEN status IN ('queued', 'running', 'ready') THEN 'expired' ELSE status END,
    file_key = '', subject_label = 'Supprimé', updated_at = now()
WHERE subject_type = $1 AND subject_id = $2;

-- name: EraseCountTimeEntriesOfUser :one
SELECT count(*) FROM time_entries WHERE user_id = $1;

-- name: EraseClientContactEmails :many
SELECT lower(email::text)::text AS email FROM contacts WHERE client_id = $1 AND email IS NOT NULL AND email::text <> '';

-- name: EraseClientCounts :one
SELECT
    (SELECT count(*) FROM contacts c WHERE c.client_id = sqlc.arg('client_id'))::bigint AS contacts,
    (SELECT count(*) FROM projects pr WHERE pr.client_id = sqlc.arg('client_id'))::bigint AS projects,
    (SELECT count(*) FROM time_entries te JOIN projects p ON p.id = te.project_id WHERE p.client_id = sqlc.arg('client_id'))::bigint AS time_entries,
    (SELECT count(*) FROM tickets t JOIN projects p ON p.id = t.project_id WHERE p.client_id = sqlc.arg('client_id'))::bigint AS tickets;

-- name: EraseDeleteClientImportTraces :execrows
-- Cles d'import et lignes de rapport qui designent le client, ses contacts,
-- ses projets ou leur temps (le libelle d'une ligne peut contenir un nom).
WITH ids AS (
    SELECT sqlc.arg('client_id')::uuid AS id
    UNION SELECT id FROM contacts WHERE client_id = sqlc.arg('client_id')
    UNION SELECT id FROM projects WHERE client_id = sqlc.arg('client_id')
    UNION SELECT te.id FROM time_entries te JOIN projects p ON p.id = te.project_id WHERE p.client_id = sqlc.arg('client_id')
), rows_gone AS (
    DELETE FROM import_rows WHERE entity_id IN (SELECT id FROM ids)
)
DELETE FROM import_keys WHERE entity_id IN (SELECT id FROM ids);

-- name: EraseDeleteClientOutbox :execrows
DELETE FROM email_outbox
WHERE lower(to_address) = ANY(sqlc.arg('emails')::text[])
   OR report_id IN (SELECT r.id FROM reports r JOIN projects p ON p.id = r.project_id WHERE p.client_id = sqlc.arg('client_id'));

-- name: EraseDeleteClientInbound :execrows
DELETE FROM inbound_emails
WHERE client_id = sqlc.arg('client_id') OR lower(from_address::text) = ANY(sqlc.arg('emails')::text[]);

-- name: EraseDeleteClientContacts :execrows
DELETE FROM contacts WHERE client_id = $1;

-- name: EraseDeleteClientProjects :execrows
-- Taches, tickets, livrables, rapports, temps et fichiers suivent par cascade.
DELETE FROM projects WHERE client_id = $1;

-- name: EraseDeleteClient :execrows
DELETE FROM clients WHERE id = $1;
