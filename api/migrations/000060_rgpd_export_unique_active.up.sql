-- Un seul export en attente ou en cours par sujet : la verification du service
-- (lire puis creer) laissait passer deux demandes simultanees.

-- Garde le plus recent des doublons eventuels ; les autres sont marques
-- interrompus, comme le fait la purge pour une construction abandonnee.
UPDATE rgpd_exports
SET status = 'failed', error = 'interrupted', updated_at = now()
WHERE status IN ('queued', 'running')
  AND id NOT IN (
      SELECT DISTINCT ON (subject_type, subject_id) id
      FROM rgpd_exports
      WHERE status IN ('queued', 'running')
      ORDER BY subject_type, subject_id, created_at DESC, id
  );

CREATE UNIQUE INDEX rgpd_exports_active_subject_idx
    ON rgpd_exports (subject_type, subject_id)
    WHERE status IN ('queued', 'running');
