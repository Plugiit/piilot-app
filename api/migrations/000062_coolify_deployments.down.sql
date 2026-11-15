-- ATTENTION : les lignes Coolify supprimees ci-dessous ne se relisent que sur
-- les 2 derniers mois ; les rapports figes gardent leurs donnees, mais un
-- deploiement plus ancien ne reviendra pas.
-- Les deploiements releves (external_id) se relisent chez Coolify : ils sont
-- retires, car leurs noms ne respecteraient pas l'unicite d'avant.
DELETE FROM deployments WHERE external_id <> '';
DROP INDEX IF EXISTS deployments_unique_external;
DROP INDEX IF EXISTS deployments_unique_name;
ALTER TABLE deployments ADD CONSTRAINT deployments_unique UNIQUE (project_id, provider, kind, name);
ALTER TABLE deployments DROP COLUMN external_id;
DROP TABLE IF EXISTS project_coolify_applications;
