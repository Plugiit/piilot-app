DROP TRIGGER time_entries_maintenance ON time_entries;
CREATE TRIGGER time_entries_maintenance
    AFTER INSERT OR DELETE OR UPDATE OF project_id, spent_on, minutes, intervention_kind, deleted_at ON time_entries
    FOR EACH ROW EXECUTE FUNCTION time_entries_maintenance();

CREATE OR REPLACE FUNCTION refresh_maintenance_month(target uuid, day date) RETURNS void LANGUAGE sql AS $$
    INSERT INTO maintenance_months (project_id, month, minutes)
    SELECT target, date_trunc('month', day)::date, coalesce(sum(minutes), 0)::integer
    FROM time_entries
    WHERE project_id = target AND deleted_at IS NULL AND intervention_kind IS NOT NULL
      AND spent_on >= date_trunc('month', day)::date
      AND spent_on < (date_trunc('month', day) + interval '1 month')::date
    ON CONFLICT (project_id, month) DO UPDATE SET minutes = EXCLUDED.minutes;
$$;

ALTER TABLE maintenance_months DROP COLUMN visible_minutes;
