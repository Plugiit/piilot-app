DELETE FROM permissions WHERE code = 'reports.write';
DROP TRIGGER IF EXISTS time_entries_maintenance ON time_entries;
DROP FUNCTION IF EXISTS time_entries_maintenance();
DROP FUNCTION IF EXISTS refresh_maintenance_month(uuid, date);
DROP TABLE IF EXISTS maintenance_months;
DROP TABLE IF EXISTS project_maintenance;
DROP TABLE IF EXISTS maintenance_plans;
DROP INDEX IF EXISTS time_entries_interventions_idx;
ALTER TABLE time_entries
    DROP CONSTRAINT IF EXISTS time_entries_visible_is_intervention,
    DROP COLUMN IF EXISTS client_note,
    DROP COLUMN IF EXISTS client_visible,
    DROP COLUMN IF EXISTS intervention_kind;
