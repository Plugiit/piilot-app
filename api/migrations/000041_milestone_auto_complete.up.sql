-- Un jalon s'atteint de lui-meme quand tous ses livrables sont valides.
--
-- C'est le declencheur qui tient les compteurs qui le fait : au moment ou le
-- dernier livrable passe « valide », le jalon prend sa date d'atteinte. Il la
-- rend si un livrable s'ajoute ou si une validation retombe — mais seulement
-- une date qu'il avait posee lui-meme : un jalon atteint a la main ne se
-- rouvre pas dans le dos de l'equipe.
ALTER TABLE milestones
    ADD COLUMN completed_by_deliverables boolean NOT NULL DEFAULT false;

CREATE OR REPLACE FUNCTION refresh_milestone_deliverable_counts(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE milestones m
    SET deliverables_total = c.total,
        deliverables_validated = c.validated,
        completed_at = CASE
            -- Tout est valide : atteint, si ce n'etait pas deja le cas.
            WHEN c.total > 0 AND c.validated = c.total THEN coalesce(m.completed_at, now())
            -- Plus tout valide : ne se rouvre que ce que le declencheur avait atteint.
            WHEN m.completed_by_deliverables THEN NULL
            ELSE m.completed_at
        END,
        completed_by_deliverables = CASE
            WHEN c.total > 0 AND c.validated = c.total THEN m.completed_at IS NULL OR m.completed_by_deliverables
            ELSE false
        END
    FROM (
        SELECT
            count(*)::integer AS total,
            count(*) FILTER (WHERE v.decision = 'valide')::integer AS validated
        FROM deliverables d
        LEFT JOIN deliverable_versions v ON v.id = d.current_version_id
        WHERE d.milestone_id = target AND d.deleted_at IS NULL
    ) c
    WHERE m.id = target;
$$;
