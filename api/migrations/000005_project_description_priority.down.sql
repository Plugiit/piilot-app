ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_priority_check;

ALTER TABLE projects
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS description;
