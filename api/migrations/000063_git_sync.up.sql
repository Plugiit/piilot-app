-- Rattrapage Git : les etiquettes des pull requests, et l'etat de la
-- synchronisation par projet.

-- Les etiquettes (labels) de la pull request : « dependencies », « security »...
-- Le webhook les remplit, et la synchronisation pour ce qu'il a manque.
ALTER TABLE pull_requests ADD COLUMN labels text[] NOT NULL DEFAULT '{}';

-- Ou en est la synchronisation d'un projet. last_attempt_at dit qu'un essai a
-- eu lieu, reussi ou non : un echec ne se retente pas en boucle, et un rapport
-- n'attend pas indefiniment. last_synced_at ne bouge qu'a un succes.
CREATE TABLE project_git_sync (
    project_id      uuid PRIMARY KEY REFERENCES projects (id) ON DELETE CASCADE,
    -- Une synchronisation demandee (bouton, rapport) : la tache la fait.
    requested_at    timestamptz,
    last_attempt_at timestamptz,
    last_synced_at  timestamptz,
    last_error      text
);
