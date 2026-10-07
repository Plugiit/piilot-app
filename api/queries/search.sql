-- Recherche globale : la palette Cmd+K.
--
-- Cinq requetes bornees a quelques lignes chacune : la palette montre les
-- premiers resultats de chaque famille, pas une liste. Les index trigram poses
-- en 000004 et 000012 servent les ILIKE sur les noms.

-- name: SearchProjects :many
SELECT p.id, p.name, p.status, p.logo_key, c.name AS client_name
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.deleted_at IS NULL
  AND p.name ILIKE '%' || sqlc.arg('q')::text || '%'
ORDER BY (p.status IN ('livre', 'hebergement')), p.name
LIMIT sqlc.arg('max_rows');

-- name: SearchClients :many
SELECT id, name, status, kind
FROM clients
WHERE deleted_at IS NULL
  AND name ILIKE '%' || sqlc.arg('q')::text || '%'
ORDER BY name
LIMIT sqlc.arg('max_rows');

-- name: SearchContacts :many
SELECT ct.id, ct.firstname, ct.lastname, ct.email, ct.client_id, coalesce(c.name, '')::text AS client_name
FROM contacts ct
LEFT JOIN clients c ON c.id = ct.client_id AND c.deleted_at IS NULL
WHERE ct.deleted_at IS NULL
  AND (
      (ct.firstname || ' ' || ct.lastname) ILIKE '%' || sqlc.arg('q')::text || '%'
      OR ct.email ILIKE '%' || sqlc.arg('q')::text || '%'
  )
ORDER BY ct.lastname, ct.firstname
LIMIT sqlc.arg('max_rows');

-- name: SearchTasks :many
SELECT t.id, t.title, t.status, t.project_id, p.name AS project_name
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND t.title ILIKE '%' || sqlc.arg('q')::text || '%'
ORDER BY (t.status = 'done'), t.created_at DESC
LIMIT sqlc.arg('max_rows');

-- name: SearchTickets :many
-- Le numero compte : « 47 » doit retrouver le ticket #47.
SELECT t.id, t.numero, t.subject, t.status, p.name AS project_name
FROM tickets t
JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
WHERE t.deleted_at IS NULL
  AND (
      t.subject ILIKE '%' || sqlc.arg('q')::text || '%'
      OR t.numero::text = sqlc.arg('q')::text
      OR ('#' || t.numero::text) = sqlc.arg('q')::text
  )
-- Le numero exact passe devant : « 3 » veut dire le ticket #3, pas ceux dont
-- le sujet contient un 3.
ORDER BY (t.numero::text = sqlc.arg('q')::text) DESC, t.updated_at DESC
LIMIT sqlc.arg('max_rows');
