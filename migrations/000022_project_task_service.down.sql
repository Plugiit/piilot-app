DROP INDEX IF EXISTS tasks_service_idx;
DROP INDEX IF EXISTS projects_service_idx;

ALTER TABLE tasks DROP COLUMN IF EXISTS service_id;
ALTER TABLE projects DROP COLUMN IF EXISTS service_id;
