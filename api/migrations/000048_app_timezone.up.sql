-- Le fuseau de l'agence, lisible depuis le SQL. Il vient d'APP_TIMEZONE,
-- recopie ici a chaque demarrage : les declencheurs qui datent un agregat
-- par jour ou par mois le lisent par app_timezone(), au lieu d'un
-- Europe/Paris en dur — Piilot est open source.
CREATE TABLE app_settings (
    id         integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    timezone   text NOT NULL DEFAULT 'Europe/Paris',
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO app_settings (id) VALUES (1);

CREATE FUNCTION app_timezone() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT coalesce((SELECT timezone FROM app_settings WHERE id = 1), 'Europe/Paris');
$$;

-- Les agregats du portail, datés dans le fuseau de l'agence.
CREATE OR REPLACE FUNCTION bump_client_activity(target uuid, at timestamptz, validated integer, resolved integer)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO client_activity_months (client_id, month, deliverables_validated, tickets_resolved)
    VALUES (target, date_trunc('month', at AT TIME ZONE app_timezone())::date, validated, resolved)
    ON CONFLICT (client_id, month) DO UPDATE SET
        deliverables_validated = client_activity_months.deliverables_validated + EXCLUDED.deliverables_validated,
        tickets_resolved = client_activity_months.tickets_resolved + EXCLUDED.tickets_resolved;

    INSERT INTO client_ticket_days (client_id, day, resolved)
    SELECT target, (at AT TIME ZONE app_timezone())::date, resolved
    WHERE resolved > 0
    ON CONFLICT (client_id, day) DO UPDATE SET
        resolved = client_ticket_days.resolved + EXCLUDED.resolved;
$$;
