-- name: SetAppTimezone :exec
-- Recopie le fuseau d'APP_TIMEZONE au demarrage, pour le SQL.
UPDATE app_settings SET timezone = sqlc.arg('timezone'), updated_at = now()
WHERE id = 1 AND timezone <> sqlc.arg('timezone');
