-- Notifications destinees a une personne.
--
-- Une ligne par destinataire et non par evenement : chacun lit et marque comme
-- lu de son cote, et l'etat de lecture n'a de sens que rapporte a quelqu'un.
-- Le meme geste produit donc autant de lignes qu'il y a de gens a prevenir —
-- c'est ce qui permet a la lecture, qui est l'operation frequente, de tenir en
-- un seul index sur le destinataire.
--
-- `actor_id` est l'auteur du geste. Il n'est jamais son propre destinataire :
-- le filtrage se fait a l'ecriture, pas a la lecture, sinon chaque ecran
-- devrait y penser.
--
-- `task_id` et `project_id` pointent la ou la notification mene. Nullables et
-- en CASCADE : un projet supprime emporte ses notifications, qui ne menent
-- plus nulle part.
--
-- `payload` porte de quoi ecrire la phrase sans relire la tache — son titre au
-- moment du geste, l'ancien et le nouveau statut. Une notification decrit ce
-- qui s'est passe, pas l'etat courant : relire la tache donnerait « a change
-- le statut en terminé » pour un geste qui l'avait mise en revue.
CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id   uuid REFERENCES users(id) ON DELETE SET NULL,
    kind       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    task_id    uuid REFERENCES tasks(id) ON DELETE CASCADE,
    project_id uuid REFERENCES projects(id) ON DELETE CASCADE,
    read_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'task_created',
        'task_status_changed',
        'task_assigned',
        'task_unassigned',
        'task_commented',
        'task_due_changed',
        'project_created'
    ));

-- L'ecran lit toujours « les miennes, les plus recentes d'abord ».
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);

-- Le compteur de non-lues est un COUNT au rendu, contrairement a la regle du
-- projet. Il est admis ici parce qu'il porte sur un index partiel restreint a
-- un seul destinataire : ce que compte la requete, c'est ce qu'elle affiche.
-- Le jour ou quelqu'un accumule des milliers de non-lues, c'est un compteur
-- tenu sur le compte qu'il faudra, pas un index de plus.
CREATE INDEX notifications_unread_idx ON notifications (user_id, created_at DESC)
    WHERE read_at IS NULL;
