-- Temps passe.
--
-- Chaque requete porte `user_id` en clause et non en filtre : on saisit et on
-- relit son propre temps, et l'identifiant vient de la session. Un ecran qui
-- accepterait un parametre laisserait lire le pointage de n'importe qui.

-- name: ListTimeEntries :many
-- Saisies d'une personne sur une plage de jours.
--
-- L'ecran montre une journee ou une semaine : les deux bornes disent laquelle,
-- et la meme requete sert les deux.
SELECT
    e.id,
    e.spent_on,
    e.minutes,
    e.note,
    e.intervention_kind,
    e.client_visible,
    e.client_note,
    e.created_at,
    p.id   AS project_id,
    p.name AS project_name,
    t.id    AS task_id,
    t.title AS task_title,
    s.id    AS service_id,
    s.name  AS service_name,
    s.color AS service_color
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
LEFT JOIN tasks t ON t.id = e.task_id AND t.deleted_at IS NULL
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.deleted_at IS NULL
  AND e.user_id = sqlc.arg('user_id')
  AND e.spent_on >= sqlc.arg('from_day')
  AND e.spent_on <= sqlc.arg('to_day')
ORDER BY e.spent_on DESC, e.created_at DESC
LIMIT sqlc.arg('page_size');

-- name: SumTimeEntries :one
-- Total de la plage, en minutes. Calcule par la base plutot qu'en additionnant
-- les lignes rendues : la liste est bornee, le total ne doit pas l'etre.
SELECT coalesce(sum(minutes), 0)::bigint
FROM time_entries
WHERE deleted_at IS NULL
  AND user_id = sqlc.arg('user_id')
  AND spent_on >= sqlc.arg('from_day')
  AND spent_on <= sqlc.arg('to_day');

-- name: SumTimeEntriesByDay :many
-- Total par jour de la plage, pour la barre de la semaine.
SELECT spent_on, coalesce(sum(minutes), 0)::bigint AS minutes
FROM time_entries
WHERE deleted_at IS NULL
  AND user_id = sqlc.arg('user_id')
  AND spent_on >= sqlc.arg('from_day')
  AND spent_on <= sqlc.arg('to_day')
GROUP BY spent_on
ORDER BY spent_on;

-- name: CreateTimeEntry :one
INSERT INTO time_entries (user_id, project_id, task_id, service_id, spent_on, minutes, note, intervention_kind, client_visible, client_note)
VALUES (
    sqlc.arg('user_id'), sqlc.arg('project_id'), sqlc.narg('task_id'),
    sqlc.narg('service_id'), sqlc.arg('spent_on'), sqlc.arg('minutes'), sqlc.arg('note'),
    sqlc.narg('intervention_kind'), sqlc.arg('client_visible'), sqlc.arg('client_note')
)
RETURNING id;

-- name: UpdateTimeEntry :one
-- La clause sur `user_id` fait le controle d'acces : une saisie qui n'est pas
-- la sienne ne correspond a aucune ligne, et l'appelant lit un 404.
UPDATE time_entries
SET project_id = sqlc.arg('project_id'),
    task_id    = sqlc.narg('task_id'),
    service_id = sqlc.narg('service_id'),
    spent_on   = sqlc.arg('spent_on'),
    minutes    = sqlc.arg('minutes'),
    note       = sqlc.arg('note'),
    intervention_kind = sqlc.narg('intervention_kind'),
    client_visible    = sqlc.arg('client_visible'),
    client_note       = sqlc.arg('client_note'),
    updated_at = now()
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id') AND deleted_at IS NULL
RETURNING id;

-- name: DeleteTimeEntry :execrows
UPDATE time_entries
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND user_id = sqlc.arg('user_id') AND deleted_at IS NULL;

-- name: GetTimeEntry :one
SELECT
    e.id,
    e.spent_on,
    e.minutes,
    e.note,
    e.intervention_kind,
    e.client_visible,
    e.client_note,
    e.created_at,
    p.id   AS project_id,
    p.name AS project_name,
    t.id    AS task_id,
    t.title AS task_title,
    s.id    AS service_id,
    s.name  AS service_name,
    s.color AS service_color
FROM time_entries e
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
LEFT JOIN tasks t ON t.id = e.task_id AND t.deleted_at IS NULL
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.id = sqlc.arg('id') AND e.user_id = sqlc.arg('user_id') AND e.deleted_at IS NULL;

-- name: FindImportUserByEmail :one
-- Le compte d'une saisie importee : un compte vivant de l'equipe ou un admin,
-- jamais un compte client.
SELECT id FROM users
WHERE email = sqlc.arg('email')::citext AND role IN ('admin', 'team') AND deleted_at IS NULL;

-- name: FindServiceByName :one
SELECT id FROM services
WHERE lower(name) = lower(sqlc.arg('name')::text) AND deleted_at IS NULL;

-- name: TimeEntryExists :one
SELECT EXISTS (SELECT 1 FROM time_entries WHERE id = $1 AND deleted_at IS NULL);

-- name: ExportTimeEntriesAfter :many
-- Une page de l'export CSV, par cle (pas d'OFFSET). Seules les saisies dont le
-- compte, le projet et le client sont vivants sortent : les autres ne se
-- reimporteraient pas.
SELECT
    e.id,
    e.spent_on,
    e.minutes,
    e.note,
    u.email  AS user_email,
    c.name   AS client_name,
    p.name   AS project_name,
    coalesce(s.name, '')::text AS service_name,
    coalesce((
        SELECT substr(k.source_key, 5) FROM import_keys k
        WHERE k.entity = 'time_entries' AND k.entity_id = e.id AND k.source_key LIKE 'ext:%'
        ORDER BY k.created_at DESC LIMIT 1
    ), '')::text AS source_key
FROM time_entries e
JOIN users u    ON u.id = e.user_id AND u.deleted_at IS NULL
JOIN projects p ON p.id = e.project_id AND p.deleted_at IS NULL
JOIN clients c  ON c.id = p.client_id AND c.deleted_at IS NULL
LEFT JOIN services s ON s.id = e.service_id AND s.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.id > sqlc.arg('after')::uuid
ORDER BY e.id
LIMIT sqlc.arg('page_size');
