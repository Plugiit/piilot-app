-- Portail client : suivi des projets et validation des livrables.

-- Un fichier de projet est interne par defaut. Le partager avec le client est
-- un geste explicite, fichier par fichier : un devis de sous-traitant ou une
-- note de cadrage ne doivent pas sortir de l'agence par accident.
ALTER TABLE attachments
    ADD COLUMN shared_with_client boolean NOT NULL DEFAULT false;

-- Le portail lit les projets d'un client, et seulement les siens : chaque
-- requete part de `client_id`.
CREATE INDEX IF NOT EXISTS projects_client_portal_idx ON projects (client_id, updated_at DESC)
    WHERE deleted_at IS NULL AND NOT is_internal;
