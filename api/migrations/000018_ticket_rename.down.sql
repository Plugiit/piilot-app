-- Les renommages deja journalises sont effaces : la contrainte d'arrivee les
-- refuserait, et une migration descendante qui echoue sur une donnee legitime
-- bloquerait le retour en arriere.
DELETE FROM ticket_events WHERE field = 'subject';

ALTER TABLE ticket_events
    DROP CONSTRAINT ticket_events_field_check;

ALTER TABLE ticket_events
    ADD CONSTRAINT ticket_events_field_check
        CHECK (field IN ('status', 'priority', 'tracker', 'assignee'));
