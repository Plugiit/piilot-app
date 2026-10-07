CREATE OR REPLACE FUNCTION refresh_milestone_deliverable_counts(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE milestones m
    SET deliverables_total = c.total,
        deliverables_validated = c.validated
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

ALTER TABLE milestones DROP COLUMN IF EXISTS completed_by_deliverables;
