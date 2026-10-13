CREATE OR REPLACE FUNCTION bump_client_activity(target uuid, at timestamptz, validated integer, resolved integer)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO client_activity_months (client_id, month, deliverables_validated, tickets_resolved)
    VALUES (target, date_trunc('month', at AT TIME ZONE 'Europe/Paris')::date, validated, resolved)
    ON CONFLICT (client_id, month) DO UPDATE SET
        deliverables_validated = client_activity_months.deliverables_validated + EXCLUDED.deliverables_validated,
        tickets_resolved = client_activity_months.tickets_resolved + EXCLUDED.tickets_resolved;

    INSERT INTO client_ticket_days (client_id, day, resolved)
    SELECT target, (at AT TIME ZONE 'Europe/Paris')::date, resolved
    WHERE resolved > 0
    ON CONFLICT (client_id, day) DO UPDATE SET
        resolved = client_ticket_days.resolved + EXCLUDED.resolved;
$$;

DROP FUNCTION IF EXISTS app_timezone();
DROP TABLE IF EXISTS app_settings;
