DROP TRIGGER IF EXISTS deliverable_versions_project_counts ON deliverable_versions;
DROP TRIGGER IF EXISTS deliverables_project_counts ON deliverables;

DROP FUNCTION IF EXISTS deliverable_versions_touch_project_counts();
DROP FUNCTION IF EXISTS deliverables_touch_project_counts();
DROP FUNCTION IF EXISTS refresh_project_deliverable_counts(uuid);

ALTER TABLE projects DROP COLUMN IF EXISTS deliverables_pending;

-- La cle croisee tombe avant les tables : sans elle, aucune des deux ne peut
-- partir en premier.
ALTER TABLE deliverables
    DROP CONSTRAINT IF EXISTS deliverables_current_version_fkey;

DROP TABLE IF EXISTS deliverable_versions;
DROP TABLE IF EXISTS deliverables;
