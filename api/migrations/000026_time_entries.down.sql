DROP TRIGGER IF EXISTS time_entries_project_hours ON time_entries;
DROP FUNCTION IF EXISTS time_entries_touch_project_hours();
DROP FUNCTION IF EXISTS refresh_project_hours_spent(uuid);

DROP TABLE IF EXISTS time_entries;

-- Les heures consommees redeviennent ce qu'elles etaient : une colonne que
-- rien n'alimente.
UPDATE projects SET hours_spent = 0;
