-- Portail client : tickets.

-- Un ticket est visible du client quand il vient de lui. Ceux que l'equipe
-- ouvre pour elle-meme sur un projet — un bug trouve en recette, une dette
-- technique — restent internes : le client n'a pas a lire le carnet de bord
-- de l'agence.
ALTER TABLE tickets
    ADD COLUMN client_visible boolean NOT NULL DEFAULT false;

UPDATE tickets t SET client_visible = true
FROM users u
WHERE u.id = t.created_by AND u.role = 'client';

-- Le portail lit les tickets visibles d'un projet, les plus recents d'abord.
CREATE INDEX tickets_client_visible_idx ON tickets (project_id, updated_at DESC)
    WHERE client_visible AND deleted_at IS NULL;

-- Pieces jointes d'un ticket : la capture d'ecran d'un bug, un document.
ALTER TABLE attachments
    ADD COLUMN ticket_id uuid REFERENCES tickets (id) ON DELETE CASCADE;

ALTER TABLE attachments DROP CONSTRAINT attachments_owner;
ALTER TABLE attachments
    ADD CONSTRAINT attachments_owner CHECK (num_nonnulls(project_id, task_id, ticket_id) = 1);

CREATE INDEX attachments_ticket_idx ON attachments (ticket_id) WHERE ticket_id IS NOT NULL;
