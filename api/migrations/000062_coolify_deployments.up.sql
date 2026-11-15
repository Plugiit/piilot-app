-- Liaison d'un projet a ses applications Coolify, et deploiements releves.

-- Les applications d'un projet, production ou preproduction. L'application
-- est designee par son uuid chez Coolify, sans cle etrangere : une application
-- qui disparait de l'instance ne detache pas le projet.
CREATE TABLE project_coolify_applications (
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    application_uuid text NOT NULL,
    environment      text NOT NULL CHECK (environment IN ('prod', 'preprod')),
    created_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, application_uuid)
);
CREATE INDEX project_coolify_applications_app ON project_coolify_applications (application_uuid);

-- L'identifiant du deploiement chez le fournisseur : il rend la releve
-- idempotente. Vide pour ce qui arrive par webhook.
ALTER TABLE deployments ADD COLUMN external_id text NOT NULL DEFAULT '';

-- Le nom ne distingue un deploiement que pour les webhooks (un tag, une
-- release). Deux deploiements Coolify d'un meme commit portent le meme nom :
-- ils se distinguent par leur identifiant.
ALTER TABLE deployments DROP CONSTRAINT deployments_unique;
CREATE UNIQUE INDEX deployments_unique_name ON deployments (project_id, provider, kind, name) WHERE external_id = '';
CREATE UNIQUE INDEX deployments_unique_external ON deployments (project_id, provider, external_id) WHERE external_id <> '';
