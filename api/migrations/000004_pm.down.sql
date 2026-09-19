-- Ordre inverse de la creation : les declencheurs et leurs fonctions d'abord,
-- puis les tables des feuilles vers la racine. Les CASCADE des cles etrangeres
-- feraient le travail, mais un DROP explicite dit ce qui disparait.
DROP TRIGGER IF EXISTS tasks_completed_at ON tasks;
DROP TRIGGER IF EXISTS tasks_project_counts ON tasks;
DROP FUNCTION IF EXISTS tasks_touch_completed_at();
DROP FUNCTION IF EXISTS tasks_touch_project_counts();
DROP FUNCTION IF EXISTS refresh_project_task_counts(uuid);

DROP TABLE IF EXISTS task_activity;
DROP TABLE IF EXISTS task_comments;
DROP TABLE IF EXISTS subtasks;
DROP TABLE IF EXISTS task_assignees;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS clients;
