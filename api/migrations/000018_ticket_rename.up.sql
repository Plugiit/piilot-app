-- Le renommage d'un ticket entre au journal.
--
-- Le sujet est ce qu'on lit dans une liste, un e-mail ou un kanban : le changer
-- change la facon dont tout le monde designe le ticket. Sans trace, personne ne
-- saurait pourquoi « la 500 du formulaire » est devenue « l'export des
-- factures ».
ALTER TABLE ticket_events
    DROP CONSTRAINT ticket_events_field_check;

ALTER TABLE ticket_events
    ADD CONSTRAINT ticket_events_field_check
        CHECK (field IN ('status', 'priority', 'tracker', 'assignee', 'subject'));
