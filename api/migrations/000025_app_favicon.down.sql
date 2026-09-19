DROP INDEX IF EXISTS sidebar_apps_favicon_todo_idx;

ALTER TABLE sidebar_apps DROP COLUMN IF EXISTS favicon_attempted_at;
ALTER TABLE sidebar_apps DROP COLUMN IF EXISTS logo_is_favicon;
