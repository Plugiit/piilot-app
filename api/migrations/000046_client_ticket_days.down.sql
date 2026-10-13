CREATE OR REPLACE FUNCTION bump_client_activity(target uuid, at timestamptz, validated integer, resolved integer)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO client_activity_months (client_id, month, deliverables_validated, tickets_resolved)
    VALUES (target, date_trunc('month', at AT TIME ZONE 'Europe/Paris')::date, validated, resolved)
    ON CONFLICT (client_id, month) DO UPDATE SET
        deliverables_validated = client_activity_months.deliverables_validated + EXCLUDED.deliverables_validated,
        tickets_resolved = client_activity_months.tickets_resolved + EXCLUDED.tickets_resolved;
$$;

DROP TABLE IF EXISTS client_ticket_days;
