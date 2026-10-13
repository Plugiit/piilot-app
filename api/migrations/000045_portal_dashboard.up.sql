-- 1.3 : tableau de bord du portail client.
--
-- Les chiffres du tableau de bord sont tenus par des declencheurs, jamais
-- recomptes a l'affichage : les demandes d'un client par etat, et ce que
-- l'agence lui a livre mois par mois.

-- Demandes visibles du client, par etat tel qu'il le lit : recues (backlog,
-- a faire), en cours (en cours, en revue), pretes a etre mises en ligne,
-- resolues. Les demandes annulees ne comptent nulle part.
CREATE TABLE client_ticket_stats (
    client_id   uuid PRIMARY KEY REFERENCES clients (id) ON DELETE CASCADE,
    received    integer NOT NULL DEFAULT 0,
    in_progress integer NOT NULL DEFAULT 0,
    ready       integer NOT NULL DEFAULT 0,
    resolved    integer NOT NULL DEFAULT 0,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Ce que l'agence a livre a un client, par mois : livrables valides et
-- demandes resolues. Une ligne par client et par mois ou il s'est passe
-- quelque chose.
CREATE TABLE client_activity_months (
    client_id              uuid NOT NULL REFERENCES clients (id) ON DELETE CASCADE,
    month                  date NOT NULL,
    deliverables_validated integer NOT NULL DEFAULT 0,
    tickets_resolved       integer NOT NULL DEFAULT 0,
    PRIMARY KEY (client_id, month)
);

-- Recompte les demandes d'un client : une poignee de lignes, a l'ecriture.
CREATE FUNCTION refresh_client_ticket_stats(target uuid) RETURNS void LANGUAGE sql AS $$
    INSERT INTO client_ticket_stats (client_id, received, in_progress, ready, resolved, updated_at)
    SELECT target,
           count(*) FILTER (WHERE t.status IN ('backlog', 'todo')),
           count(*) FILTER (WHERE t.status IN ('in_progress', 'in_review')),
           count(*) FILTER (WHERE t.status = 'ready_to_deploy'),
           count(*) FILTER (WHERE t.status = 'done'),
           now()
    FROM tickets t
    JOIN projects p ON p.id = t.project_id AND p.deleted_at IS NULL
    WHERE p.client_id = target AND t.client_visible AND t.deleted_at IS NULL
    ON CONFLICT (client_id) DO UPDATE SET
        received = EXCLUDED.received, in_progress = EXCLUDED.in_progress,
        ready = EXCLUDED.ready, resolved = EXCLUDED.resolved, updated_at = now();
$$;

CREATE FUNCTION bump_client_activity(target uuid, at timestamptz, validated integer, resolved integer)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO client_activity_months (client_id, month, deliverables_validated, tickets_resolved)
    VALUES (target, date_trunc('month', at AT TIME ZONE 'Europe/Paris')::date, validated, resolved)
    ON CONFLICT (client_id, month) DO UPDATE SET
        deliverables_validated = client_activity_months.deliverables_validated + EXCLUDED.deliverables_validated,
        tickets_resolved = client_activity_months.tickets_resolved + EXCLUDED.tickets_resolved;
$$;

CREATE FUNCTION tickets_client_stats() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    old_client uuid;
    new_client uuid;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        SELECT client_id INTO old_client FROM projects WHERE id = OLD.project_id;
    END IF;
    IF TG_OP <> 'DELETE' THEN
        SELECT client_id INTO new_client FROM projects WHERE id = NEW.project_id;
    END IF;

    IF old_client IS NOT NULL THEN
        PERFORM refresh_client_ticket_stats(old_client);
    END IF;
    IF new_client IS NOT NULL AND new_client IS DISTINCT FROM old_client THEN
        PERFORM refresh_client_ticket_stats(new_client);
    END IF;

    -- Une demande visible qui passe « resolue » compte pour le mois.
    IF TG_OP = 'UPDATE' AND new_client IS NOT NULL AND NEW.client_visible
       AND NEW.status = 'done' AND OLD.status <> 'done' THEN
        PERFORM bump_client_activity(new_client, now(), 0, 1);
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER tickets_client_stats
    AFTER INSERT OR DELETE OR UPDATE OF status, client_visible, deleted_at, project_id ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_client_stats();

CREATE FUNCTION deliverable_versions_client_activity() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    target uuid;
BEGIN
    IF NEW.decision = 'valide' AND OLD.decision IS DISTINCT FROM 'valide' THEN
        SELECT p.client_id INTO target
        FROM deliverables d JOIN projects p ON p.id = d.project_id
        WHERE d.id = NEW.deliverable_id;
        IF target IS NOT NULL THEN
            PERFORM bump_client_activity(target, coalesce(NEW.decided_at, now()), 1, 0);
        END IF;
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER deliverable_versions_client_activity
    AFTER UPDATE OF decision ON deliverable_versions
    FOR EACH ROW EXECUTE FUNCTION deliverable_versions_client_activity();

-- Reprise de l'existant.
SELECT refresh_client_ticket_stats(id) FROM clients;

INSERT INTO client_activity_months (client_id, month, deliverables_validated, tickets_resolved)
SELECT client_id, month, sum(validated)::integer, sum(resolved)::integer
FROM (
    SELECT p.client_id, date_trunc('month', v.decided_at AT TIME ZONE 'Europe/Paris')::date AS month, 1 AS validated, 0 AS resolved
    FROM deliverable_versions v
    JOIN deliverables d ON d.id = v.deliverable_id
    JOIN projects p ON p.id = d.project_id
    WHERE v.decision = 'valide' AND v.decided_at IS NOT NULL
    UNION ALL
    SELECT p.client_id, date_trunc('month', e.created_at AT TIME ZONE 'Europe/Paris')::date, 0, 1
    FROM ticket_events e
    JOIN tickets t ON t.id = e.ticket_id AND t.client_visible
    JOIN projects p ON p.id = t.project_id
    WHERE e.field = 'status' AND e.new_value = 'done'
) src
GROUP BY client_id, month;
