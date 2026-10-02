-- Espace team : ce qu'un compte de l'equipe voit, et ce qu'il apprend.

-- Trois permissions pour ce qui relevait jusqu'ici du simple fait d'etre dans
-- le back-office. Le tableau de bord de l'agence, les budgets (heures vendues
-- et alertes) et le pipeline commercial sont des outils de direction : un chef
-- de projet n'en a pas besoin pour faire son travail, et les y voir brouille
-- son espace.
INSERT INTO permissions (code, label) VALUES
    ('dashboard.read', 'Consulter le tableau de bord de l''agence'),
    ('budgets.read',   'Consulter et modifier les budgets des projets'),
    ('pipeline.read',  'Suivre et faire avancer le pipeline commercial');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code IN ('dashboard.read', 'budgets.read', 'pipeline.read');

-- `time.read` ne servait qu'aux rapports, qui lisent le temps de toute
-- l'equipe : il prend ce nom, et quitte le role team. Chacun continue de lire
-- et de saisir le sien, sous `time.write`.
UPDATE permissions SET label = 'Consulter les rapports de temps de l''équipe'
WHERE code = 'time.read';

-- Les roles et permissions sont un ecran d'administration : l'equipe n'a pas a
-- le parcourir. `users.read` reste, il sert a choisir qui affecter a une
-- tache.
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE code = 'team')
  AND permission_id IN (SELECT id FROM permissions WHERE code IN ('time.read', 'roles.read'));

-- Notifications des tickets et des livrables. Elles pointent la ou elles
-- menent, comme celles des taches ; un ticket ou un livrable supprime emporte
-- les siennes.
ALTER TABLE notifications
    ADD COLUMN ticket_id      uuid REFERENCES tickets (id) ON DELETE CASCADE,
    ADD COLUMN deliverable_id uuid REFERENCES deliverables (id) ON DELETE CASCADE;

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'task_created',
        'task_status_changed',
        'task_assigned',
        'task_unassigned',
        'task_commented',
        'task_due_changed',
        'project_created',
        'ticket_created',
        'ticket_assigned',
        'ticket_replied',
        'ticket_status_changed',
        'deliverable_validated',
        'deliverable_feedback'
    ));
