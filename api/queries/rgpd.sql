-- Lectures de l'export RGPD, une par section de l'archive.
--
-- Toutes rendent (id, data) : `data` est la ligne en JSON, et la pagination se
-- fait par cle (`id > after ORDER BY id`), jamais par OFFSET, pour que
-- l'archive se construise sans tout garder en memoire. Les colonnes secretes
-- (mot de passe, TOTP, empreintes de jetons, courrier brut) ne sortent pas : les
-- tables qui en portent listent leurs colonnes une a une plutot que de
-- retirer celles qu'on connait aujourd'hui.

-- name: RgpdUserProfile :many
-- Compte : le profil, sans mot de passe ni secret TOTP.
SELECT x.id AS id, jsonb_build_object('id', x.id, 'email', x.email, 'firstname', x.firstname, 'lastname', x.lastname, 'role', x.role, 'gender', x.gender, 'phone', x.phone, 'address', x.address, 'postal_code', x.postal_code, 'city', x.city, 'country', x.country, 'avatar_url', x.avatar_url, 'booking_url', x.booking_url, 'client_id', x.client_id, 'last_login_at', x.last_login_at, 'created_at', x.created_at, 'updated_at', x.updated_at, 'disabled_at', x.disabled_at, 'deleted_at', x.deleted_at) AS data
FROM users x
WHERE x.id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserSessions :many
-- Compte : les sessions (sans l'empreinte du jeton).
SELECT x.id AS id, jsonb_build_object('id', x.id, 'user_agent', x.user_agent, 'ip', host(x.ip), 'expires_at', x.expires_at, 'revoked_at', x.revoked_at, 'created_at', x.created_at) AS data
FROM refresh_tokens x
WHERE x.user_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserTimeEntries :many
-- Compte : son temps passe.
SELECT x.id AS id, to_jsonb(x) AS data
FROM time_entries x
WHERE x.user_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserTasks :many
-- Compte : les taches qu'il a creees ou qui lui sont confiees.
SELECT x.id AS id, to_jsonb(x) AS data
FROM tasks x
WHERE (x.created_by = sqlc.arg('subject')::uuid OR EXISTS (SELECT 1 FROM task_assignees a WHERE a.task_id = x.id AND a.user_id = sqlc.arg('subject')::uuid)) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserTaskComments :many
-- Compte : ses commentaires de taches.
SELECT x.id AS id, to_jsonb(x) AS data
FROM task_comments x
WHERE x.author_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserTicketMessages :many
-- Compte : ses messages de tickets, notes internes comprises (ce sont les siens).
SELECT x.id AS id, to_jsonb(x) AS data
FROM ticket_messages x
WHERE x.author_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserNotifications :many
-- Compte : ses notifications.
SELECT x.id AS id, to_jsonb(x) AS data
FROM notifications x
WHERE x.user_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserOauthGrants :many
-- Compte : ses connexions d'outils (Claude), sans jeton.
SELECT x.id AS id, jsonb_build_object('id', x.id, 'client_id', x.client_id, 'scopes', x.scopes, 'created_at', x.created_at, 'last_used_at', x.last_used_at, 'revoked_at', x.revoked_at) AS data
FROM oauth_grants x
WHERE x.user_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserAudit :many
-- Compte : les lignes du journal d'audit dont il est l'acteur.
SELECT x.id AS id, to_jsonb(x) AS data
FROM audit_log x
WHERE x.actor_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientRecord :many
-- Client : la fiche.
SELECT x.id AS id, to_jsonb(x) AS data
FROM clients x
WHERE x.id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientContacts :many
-- Client : ses contacts.
SELECT x.id AS id, to_jsonb(x) AS data
FROM contacts x
WHERE x.client_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientPortalUsers :many
-- Client : ses comptes du portail, sans mot de passe ni secret TOTP.
SELECT x.id AS id, jsonb_build_object('id', x.id, 'email', x.email, 'firstname', x.firstname, 'lastname', x.lastname, 'role', x.role, 'gender', x.gender, 'phone', x.phone, 'address', x.address, 'postal_code', x.postal_code, 'city', x.city, 'country', x.country, 'avatar_url', x.avatar_url, 'booking_url', x.booking_url, 'client_id', x.client_id, 'last_login_at', x.last_login_at, 'created_at', x.created_at, 'updated_at', x.updated_at, 'disabled_at', x.disabled_at, 'deleted_at', x.deleted_at) AS data
FROM users x
WHERE x.client_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientProjects :many
-- Client : ses projets.
SELECT x.id AS id, to_jsonb(x) AS data
FROM projects x
WHERE x.client_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientMilestones :many
-- Client : les jalons de ses projets.
SELECT x.id AS id, to_jsonb(x) AS data
FROM milestones x
WHERE x.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientDeliverables :many
-- Client : les livrables de ses projets.
SELECT x.id AS id, to_jsonb(x) AS data
FROM deliverables x
WHERE x.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientDeliverableVersions :many
-- Client : les versions de ces livrables.
SELECT x.id AS id, to_jsonb(x) AS data
FROM deliverable_versions x
WHERE x.deliverable_id IN (SELECT d.id FROM deliverables d JOIN projects p ON p.id = d.project_id WHERE p.client_id = sqlc.arg('subject')::uuid) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientTickets :many
-- Client : les tickets de ses projets.
SELECT x.id AS id, to_jsonb(x) AS data
FROM tickets x
WHERE x.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientTicketMessages :many
-- Client : les messages de ces tickets. Les notes internes ne sortent que si with_internal est vrai (decision de l'appelant, un seul point : usecase.rgpdIncludeInternalNotes).
SELECT x.id AS id, to_jsonb(x) AS data
FROM ticket_messages x
WHERE x.ticket_id IN (SELECT t.id FROM tickets t JOIN projects p ON p.id = t.project_id WHERE p.client_id = sqlc.arg('subject')::uuid) AND (NOT x.is_internal OR sqlc.arg('with_internal')::boolean) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientInteractions :many
-- Client : son historique d'echanges.
SELECT x.id AS id, to_jsonb(x) AS data
FROM client_interactions x
WHERE x.client_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientInboundEmails :many
-- Client : les e-mails recus (metadonnees, sans le message brut).
SELECT x.id AS id, (to_jsonb(x) - 'raw'::text)::jsonb AS data
FROM inbound_emails x
WHERE x.client_id = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientReports :many
-- Client : les rapports mensuels de ses projets.
SELECT x.id AS id, to_jsonb(x) AS data
FROM reports x
WHERE x.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientTimeEntries :many
-- Client : le temps passe sur ses projets.
SELECT x.id AS id, to_jsonb(x) AS data
FROM time_entries x
WHERE x.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid) AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientAttachments :many
-- Client : la liste des fichiers de ses projets, de leurs taches et de leurs tickets.
SELECT a.id AS id, (to_jsonb(a) - 'storage_key'::text)::jsonb AS data
FROM attachments a
WHERE (a.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid)
       OR a.task_id IN (SELECT t.id FROM tasks t JOIN projects p ON p.id = t.project_id WHERE p.client_id = sqlc.arg('subject')::uuid)
       OR a.ticket_id IN (SELECT t.id FROM tickets t JOIN projects p ON p.id = t.project_id WHERE p.client_id = sqlc.arg('subject')::uuid)) AND a.id > sqlc.arg('after')::uuid
ORDER BY a.id
LIMIT sqlc.arg('page_size');

-- name: RgpdClientFiles :many
-- Client : TOUS les fichiers du client (pieces jointes des projets, des taches
-- et des tickets, plus le logo de chaque projet). Source unique : l'archive
-- d'export les ajoute, et l'effacement releve ses cles ici pour les supprimer.
SELECT f.id, f.filename, f.storage_key FROM (
    SELECT a.id, a.filename, a.storage_key FROM attachments a WHERE (a.project_id IN (SELECT p.id FROM projects p WHERE p.client_id = sqlc.arg('subject')::uuid)
       OR a.task_id IN (SELECT t.id FROM tasks t JOIN projects p ON p.id = t.project_id WHERE p.client_id = sqlc.arg('subject')::uuid)
       OR a.ticket_id IN (SELECT t.id FROM tickets t JOIN projects p ON p.id = t.project_id WHERE p.client_id = sqlc.arg('subject')::uuid))
    UNION ALL
    SELECT p.id, ('logo-' || p.id::text) AS filename, p.logo_key AS storage_key
    FROM projects p
    WHERE p.client_id = sqlc.arg('subject')::uuid AND p.logo_key IS NOT NULL AND p.logo_key <> ''
) f
WHERE f.id > sqlc.arg('after')::uuid
ORDER BY f.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserFiles :many
-- Compte : les fichiers qu'il a deposes.
SELECT x.id, x.filename, x.storage_key
FROM attachments x
WHERE x.uploaded_by = sqlc.arg('subject')::uuid
  AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserAttachments :many
-- Compte : la liste des fichiers qu'il a deposes.
SELECT x.id AS id, (to_jsonb(x) - 'storage_key'::text)::jsonb AS data
FROM attachments x
WHERE x.uploaded_by = sqlc.arg('subject')::uuid AND x.id > sqlc.arg('after')::uuid
ORDER BY x.id
LIMIT sqlc.arg('page_size');

-- name: RgpdUserLabel :one
SELECT btrim(firstname || ' ' || lastname) AS name, email::text AS email
FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: RgpdClientLabel :one
-- Un client supprime dans le CRM (suppression logique) garde toutes ses
-- donnees : il reste exportable et effacable.
SELECT name FROM clients WHERE id = $1;

-- name: ListDeletedClients :many
-- Les clients supprimes dans le CRM mais dont les donnees sont toujours la : le
-- CRM ne les montre plus, l'ecran RGPD doit pouvoir les retrouver.
SELECT id, name, deleted_at FROM clients
WHERE deleted_at IS NOT NULL
ORDER BY deleted_at DESC, id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountDeletedClients :one
SELECT count(*) FROM clients WHERE deleted_at IS NOT NULL;

-- name: CreateRgpdExport :one
INSERT INTO rgpd_exports (subject_type, subject_id, subject_label, requested_by)
VALUES (sqlc.arg('subject_type'), sqlc.arg('subject_id'), sqlc.arg('subject_label'), sqlc.narg('requested_by'))
RETURNING *;

-- name: GetRgpdExport :one
SELECT * FROM rgpd_exports WHERE id = $1;

-- name: ListRgpdExports :many
SELECT * FROM rgpd_exports
ORDER BY created_at DESC, id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountRgpdExports :one
SELECT count(*) FROM rgpd_exports;

-- name: ClaimQueuedRgpdExport :one
-- Le prochain export a construire. Une seule instruction : le verrou
-- SKIP LOCKED suffit a ce que deux instances ne prennent pas le meme.
UPDATE rgpd_exports
SET status = 'running', updated_at = now()
WHERE id = (
    SELECT e.id FROM rgpd_exports e
    WHERE e.status = 'queued'
    ORDER BY e.created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: FinishRgpdExport :execrows
-- Rend 0 si l'export n'est plus « running » : l'appelant retire alors l'archive.
UPDATE rgpd_exports
SET status = 'ready', file_key = sqlc.arg('file_key'), size_bytes = sqlc.arg('size_bytes'),
    ready_at = now(), expires_at = sqlc.arg('expires_at'), error = '', updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'running';

-- name: FailRgpdExport :exec
UPDATE rgpd_exports
SET status = 'failed', error = sqlc.arg('error'), file_key = '', updated_at = now()
WHERE id = sqlc.arg('id') AND status = 'running';

-- name: FailStaleRgpdExports :many
-- Un export « en cours » qui ne donne plus signe de vie : son instance s'est
-- arretee. Il n'est pas rejoue tout seul, l'admin en redemande un.
UPDATE rgpd_exports
SET status = 'failed', error = 'interrupted', updated_at = now()
WHERE status = 'running' AND updated_at < now() - interval '30 minutes'
RETURNING id;

-- name: ListExpiredRgpdExports :many
-- Archives arrivees a echeance. Le fichier est supprime AVANT de marquer
-- l'export : un arret entre les deux laisse l'export « ready », repris au
-- tour suivant, plutot qu'une archive de donnees personnelles orpheline.
SELECT id, file_key FROM rgpd_exports
WHERE status = 'ready' AND expires_at <= now()
ORDER BY expires_at
LIMIT 50;

-- name: MarkRgpdExportExpired :exec
UPDATE rgpd_exports
SET status = 'expired', file_key = '', updated_at = now()
WHERE id = $1 AND status = 'ready';

-- name: GetActiveRgpdExport :one
-- Un export deja demande, pas encore fini, pour ce sujet : une seconde demande
-- le retrouve au lieu de lancer une seconde construction.
SELECT * FROM rgpd_exports
WHERE subject_type = sqlc.arg('subject_type') AND subject_id = sqlc.arg('subject_id')
  AND status IN ('queued', 'running')
ORDER BY created_at
LIMIT 1;
