-- Le temps visible par le client, a cote du temps consomme : le portail dit
-- « dont N min d'interventions internes » sans rien recompter a l'affichage.
ALTER TABLE maintenance_months ADD COLUMN visible_minutes integer NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION refresh_maintenance_month(target uuid, day date) RETURNS void LANGUAGE sql AS $$
    INSERT INTO maintenance_months (project_id, month, minutes, visible_minutes)
    SELECT target, date_trunc('month', day)::date,
           coalesce(sum(minutes), 0)::integer,
           coalesce(sum(minutes) FILTER (WHERE client_visible), 0)::integer
    FROM time_entries
    WHERE project_id = target AND deleted_at IS NULL AND intervention_kind IS NOT NULL
      AND spent_on >= date_trunc('month', day)::date
      AND spent_on < (date_trunc('month', day) + interval '1 month')::date
    ON CONFLICT (project_id, month) DO UPDATE SET minutes = EXCLUDED.minutes, visible_minutes = EXCLUDED.visible_minutes;
$$;

-- La visibilite change desormais l'agregat : le declencheur la suit.
DROP TRIGGER time_entries_maintenance ON time_entries;
CREATE TRIGGER time_entries_maintenance
    AFTER INSERT OR DELETE OR UPDATE OF project_id, spent_on, minutes, intervention_kind, client_visible, deleted_at ON time_entries
    FOR EACH ROW EXECUTE FUNCTION time_entries_maintenance();

-- Reprise des mois existants.
SELECT refresh_maintenance_month(project_id, month) FROM maintenance_months;
