-- Livrables.
--
-- L'etat d'un livrable n'est pas une colonne : c'est la decision de sa version
-- courante, et l'absence de version vaut brouillon. Toutes les requetes qui
-- l'affichent joignent donc `deliverable_versions` sur `current_version_id` —
-- une jointure sur cle primaire, pas un LATERAL par ligne.

-- name: ListDeliverables :many
-- L'ecran « Livrables » du module, qui traverse les projets.
--
-- Trie du plus recemment soumis au plus ancien, les brouillons en queue : c'est
-- l'ordre dans lequel on reprend un dossier. L'anciennete d'une attente se lit
-- sur `submitted_at`, que l'ecran compare a maintenant — une duree renvoyee ici
-- serait fausse des la seconde suivante.
SELECT
    d.id,
    d.title,
    d.created_at,
    coalesce(v.decision, 'brouillon') AS status,
    p.id   AS project_id,
    p.name AS project_name,
    c.id   AS client_id,
    c.name AS client_name,
    v.id           AS version_id,
    v.numero       AS version_numero,
    v.url          AS version_url,
    v.attachment_id,
    v.submitted_at,
    v.decided_at,
    v.feedback,
    s.id         AS submitter_id,
    s.firstname  AS submitter_firstname,
    s.lastname   AS submitter_lastname,
    s.avatar_url AS submitter_avatar_url
FROM deliverables d
JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL
JOIN clients c ON c.id = p.client_id AND c.deleted_at IS NULL
LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
LEFT JOIN users s ON s.id = v.submitted_by AND s.deleted_at IS NULL
WHERE d.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL
       OR coalesce(v.decision, 'brouillon') = sqlc.narg('status')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR d.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL
       OR d.title ILIKE '%' || sqlc.narg('search')::text || '%')
ORDER BY v.submitted_at DESC NULLS LAST, d.created_at DESC, d.id
LIMIT sqlc.arg('page_size') OFFSET sqlc.arg('page_offset');

-- name: CountDeliverables :one
-- Total pour la pagination, aux memes conditions que la liste.
SELECT count(*)
FROM deliverables d
JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL
JOIN clients c ON c.id = p.client_id AND c.deleted_at IS NULL
LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
WHERE d.deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL
       OR coalesce(v.decision, 'brouillon') = sqlc.narg('status')::text)
  AND (sqlc.narg('project_id')::uuid IS NULL OR d.project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('search')::text IS NULL
       OR d.title ILIKE '%' || sqlc.narg('search')::text || '%');

-- name: GetDeliverable :one
-- Un livrable et sa version courante, aux memes colonnes que la liste : le
-- depot et la decision rendent la ligne telle que l'ecran la reaffiche.
SELECT
    d.id,
    d.title,
    d.created_at,
    coalesce(v.decision, 'brouillon') AS status,
    p.id   AS project_id,
    p.name AS project_name,
    c.id   AS client_id,
    c.name AS client_name,
    v.id           AS version_id,
    v.numero       AS version_numero,
    v.url          AS version_url,
    v.attachment_id,
    v.submitted_at,
    v.decided_at,
    v.feedback,
    s.id         AS submitter_id,
    s.firstname  AS submitter_firstname,
    s.lastname   AS submitter_lastname,
    s.avatar_url AS submitter_avatar_url
FROM deliverables d
JOIN projects p ON p.id = d.project_id AND p.deleted_at IS NULL
JOIN clients c ON c.id = p.client_id AND c.deleted_at IS NULL
LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
LEFT JOIN users s ON s.id = v.submitted_by AND s.deleted_at IS NULL
WHERE d.id = sqlc.arg('id') AND d.deleted_at IS NULL;

-- name: ListDeliverableVersions :many
-- Le fil complet d'un livrable, de la premiere version a la derniere : c'est
-- la trace que le module existe pour garder.
SELECT
    v.id,
    v.numero,
    v.url,
    v.attachment_id,
    v.submitted_at,
    v.decision,
    v.decided_at,
    v.feedback,
    s.id         AS submitter_id,
    s.firstname  AS submitter_firstname,
    s.lastname   AS submitter_lastname,
    s.avatar_url AS submitter_avatar_url,
    r.id         AS decider_id,
    r.firstname  AS decider_firstname,
    r.lastname   AS decider_lastname,
    r.avatar_url AS decider_avatar_url,
    -- A quel titre la decision a ete rendue : le client lui-meme, ou l'agence
    -- qui enregistre une reponse recue ailleurs. Les deux portent le droit.
    r.role       AS decider_role
FROM deliverable_versions v
LEFT JOIN users s ON s.id = v.submitted_by AND s.deleted_at IS NULL
LEFT JOIN users r ON r.id = v.decided_by AND r.deleted_at IS NULL
WHERE v.deliverable_id = sqlc.arg('deliverable_id')
ORDER BY v.numero;

-- name: CreateDeliverable :one
-- Le livrable nait sans version : c'est la soumission qui lui en donne une.
INSERT INTO deliverables (project_id, title, description, created_by)
VALUES (
    sqlc.arg('project_id'), sqlc.arg('title'),
    sqlc.arg('description'), sqlc.narg('created_by')
)
RETURNING id;

-- name: LockDeliverable :one
-- Verrou avant d'ajouter une version ou de trancher : deux soumissions
-- simultanees prendraient sinon le meme numero.
SELECT id, project_id, current_version_id
FROM deliverables
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
FOR UPDATE;

-- name: NextDeliverableVersionNumero :one
-- Le numero suivant, sous le verrou pose juste avant.
SELECT coalesce(max(numero), 0) + 1
FROM deliverable_versions
WHERE deliverable_id = sqlc.arg('deliverable_id');

-- name: CreateDeliverableVersion :one
INSERT INTO deliverable_versions (
    deliverable_id, numero, attachment_id, url, submitted_by
)
VALUES (
    sqlc.arg('deliverable_id'), sqlc.arg('numero'),
    sqlc.narg('attachment_id'), sqlc.arg('url'), sqlc.narg('submitted_by')
)
RETURNING id;

-- name: SetDeliverableCurrentVersion :exec
-- La version qui vient d'etre soumise devient celle que l'ecran montre.
UPDATE deliverables
SET current_version_id = sqlc.arg('version_id'),
    updated_at = now()
WHERE id = sqlc.arg('id');

-- name: DecideDeliverableVersion :exec
-- Enregistre la reponse du client sur une version precise.
--
-- La clause sur `decision` rend la requete rejouable sans degat : une decision
-- deja prise ne se reecrit pas, et l'appelant lit zero ligne touchee plutot que
-- d'ecraser la date et l'auteur d'origine.
UPDATE deliverable_versions
SET decision   = sqlc.arg('decision'),
    decided_at = now(),
    decided_by = sqlc.narg('decided_by'),
    feedback   = sqlc.arg('feedback')
WHERE id = sqlc.arg('id') AND decision = 'en_attente';

-- name: TouchDeliverable :exec
UPDATE deliverables SET updated_at = now() WHERE id = sqlc.arg('id');
