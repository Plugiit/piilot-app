DROP TRIGGER IF EXISTS deliverable_versions_daily_activity ON deliverable_versions;
DROP TRIGGER IF EXISTS tickets_daily_activity ON tickets;
DROP TRIGGER IF EXISTS tasks_daily_activity_update ON tasks;
DROP TRIGGER IF EXISTS tasks_daily_activity_insert ON tasks;
DROP FUNCTION IF EXISTS deliverable_versions_touch_daily_activity();
DROP FUNCTION IF EXISTS tickets_touch_daily_activity();
DROP FUNCTION IF EXISTS tasks_touch_daily_activity();
DROP FUNCTION IF EXISTS bump_daily_activity(date, integer, integer, integer);
DROP TABLE IF EXISTS daily_activity;
