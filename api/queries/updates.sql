-- name: GetReleaseCheck :one
SELECT * FROM app_release_check WHERE id;

-- name: SaveReleaseCheck :exec
-- Resultat d'un passage reussi : la version vue remplace la precedente.
INSERT INTO app_release_check (id, version, name, url, published_at, checked_at, error)
VALUES (true, $1, $2, $3, $4, now(), '')
ON CONFLICT (id) DO UPDATE SET
    version      = EXCLUDED.version,
    name         = EXCLUDED.name,
    url          = EXCLUDED.url,
    published_at = EXCLUDED.published_at,
    checked_at   = now(),
    error        = '';

-- name: SaveReleaseCheckError :exec
-- Passage rate : on garde la derniere version connue, on note l'erreur.
INSERT INTO app_release_check (id, checked_at, error)
VALUES (true, now(), $1)
ON CONFLICT (id) DO UPDATE SET checked_at = now(), error = EXCLUDED.error;

-- name: GetUpdater :one
SELECT * FROM app_updater WHERE id;

-- name: SaveUpdaterHeartbeat :exec
INSERT INTO app_updater (id, version, target, error, last_seen)
VALUES (true, $1, $2, $3, now())
ON CONFLICT (id) DO UPDATE SET
    version   = EXCLUDED.version,
    target    = EXCLUDED.target,
    error     = EXCLUDED.error,
    last_seen = now();

-- name: CreateUpdateRequest :one
INSERT INTO app_update_requests (requested_by, from_version, target_version)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetLatestUpdateRequest :one
-- Derniere demande, pour l'etat affiche a l'ecran.
SELECT r.*, coalesce(btrim(u.firstname || ' ' || u.lastname), '')::text AS requested_by_name
FROM app_update_requests r
LEFT JOIN users u ON u.id = r.requested_by
ORDER BY r.created_at DESC
LIMIT 1;

-- name: StartPendingUpdateRequest :one
-- L'updater prend la demande en attente et la passe en cours, d'un seul
-- geste : une demande n'est jamais executee deux fois.
UPDATE app_update_requests
SET status = 'running', started_at = now(), step = 'Démarrage'
WHERE id = (
    SELECT id FROM app_update_requests
    WHERE status = 'pending'
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: SetUpdateRequestStep :exec
UPDATE app_update_requests SET step = $2 WHERE id = $1 AND status = 'running';

-- name: FinishUpdateRequest :exec
UPDATE app_update_requests
SET status = $2, error = $3, finished_at = now()
WHERE id = $1;

-- name: AbandonStaleUpdateRequests :exec
-- Au demarrage de l'updater : une demande restee « en cours » vient d'un
-- updater interrompu en pleine mise a jour. Elle est close en echec plutot que
-- de bloquer toute nouvelle demande.
UPDATE app_update_requests
SET status = 'failed', error = 'Interrompue : l''updater a redémarré pendant la mise à jour', finished_at = now()
WHERE status = 'running';
