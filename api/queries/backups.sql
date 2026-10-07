-- Sauvegardes : la commande `backup` ecrit ici, l'ecran des parametres et la
-- tache de fond qui previent les admins y lisent.

-- name: CreateBackup :one
INSERT INTO app_backups (status) VALUES ('running')
RETURNING *;

-- name: FinishBackup :exec
UPDATE app_backups
SET finished_at = now(),
    status      = sqlc.arg('status'),
    location    = sqlc.arg('location'),
    size_bytes  = sqlc.arg('size_bytes'),
    error       = sqlc.arg('error')
WHERE id = sqlc.arg('id');

-- name: AbandonStaleBackups :exec
-- Une sauvegarde restee « en cours » plus de six heures vient d'une commande
-- interrompue : elle ne doit pas passer pour en route.
UPDATE app_backups
SET status = 'failed', finished_at = now(), error = 'Interrompue'
WHERE status = 'running' AND started_at < now() - interval '6 hours';

-- name: ListBackups :many
SELECT * FROM app_backups
ORDER BY started_at DESC
LIMIT 20;

-- name: GetLastSuccessfulBackup :one
SELECT * FROM app_backups
WHERE status = 'done'
ORDER BY started_at DESC
LIMIT 1;

-- name: PruneBackupRows :exec
-- Le journal n'a pas a remonter plus loin que la retention la plus longue.
DELETE FROM app_backups WHERE started_at < now() - interval '120 days';

-- name: LastNotificationOfKind :one
-- Quand une alerte globale a ete emise pour la derniere fois, toutes
-- personnes confondues : une alerte par jour suffit.
SELECT coalesce(max(created_at), to_timestamp(0))::timestamptz AS at
FROM notifications
WHERE kind = sqlc.arg('kind');
