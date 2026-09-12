DROP TABLE IF EXISTS project_favorites;
DROP TABLE IF EXISTS project_files;

ALTER TABLE projects
    DROP COLUMN IF EXISTS external_url;

ALTER TABLE tasks
    DROP CONSTRAINT IF EXISTS tasks_priority_check;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS priority;
