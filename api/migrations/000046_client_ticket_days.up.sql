-- Demandes resolues par jour et par client : la courbe du tableau de bord du
-- portail. Le mois etait trop gros grain — un client dont toutes les
-- demandes tombent dans le meme mois n'y voyait qu'un point.
CREATE TABLE client_ticket_days (
    client_id uuid NOT NULL REFERENCES clients (id) ON DELETE CASCADE,
    day       date NOT NULL,
    resolved  integer NOT NULL DEFAULT 0,
    PRIMARY KEY (client_id, day)
);

-- Le meme point d'entree que le mois : une resolution compte aussi pour son
-- jour.
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

-- Reprise de l'existant, depuis l'historique des statuts.
INSERT INTO client_ticket_days (client_id, day, resolved)
SELECT p.client_id, (e.created_at AT TIME ZONE 'Europe/Paris')::date, count(*)::integer
FROM ticket_events e
JOIN tickets t ON t.id = e.ticket_id AND t.client_visible
JOIN projects p ON p.id = t.project_id
WHERE e.field = 'status' AND e.new_value = 'done' AND p.client_id IS NOT NULL
GROUP BY 1, 2;
