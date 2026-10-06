-- Detection des nouvelles versions, pour toutes les installations.
--
-- Aucune instance auto-hebergee ne peut recevoir d'appel de GitHub sans qu'on
-- configure le depot pour elle. Chacune interroge donc GitHub elle-meme,
-- souvent, en requete conditionnelle : quand rien n'a change, GitHub repond
-- 304, ce qui ne compte pas dans sa limite d'appels.

-- Empreinte de la derniere reponse, renvoyee a l'appel suivant.
ALTER TABLE app_release_check
    ADD COLUMN etag text NOT NULL DEFAULT '',
    -- Un admin a demande une verification immediate : la tache de fond la
    -- fait a son prochain passage, dans les quinze secondes. L'appel a GitHub ne part
    -- jamais pendant la requete de l'admin.
    ADD COLUMN check_requested_at timestamptz,
    -- Derniere version annoncee aux admins dans la cloche : une version n'est
    -- annoncee qu'une fois, quel que soit le nombre de passages.
    ADD COLUMN notified_version text NOT NULL DEFAULT '';

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
        'deliverable_feedback',
        'update_available'
    ));
