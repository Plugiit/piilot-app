ALTER TABLE clients
    ADD COLUMN contact_name  text NOT NULL DEFAULT '',
    ADD COLUMN contact_role  text NOT NULL DEFAULT '',
    ADD COLUMN contact_email citext;

-- Retour des interlocuteurs dans les colonnes plates, recomposes depuis le
-- contact principal. Les contacts qui n'etaient principaux de personne sont
-- perdus : les colonnes n'en tiennent qu'un, c'est la raison d'etre de la
-- migration montante.
UPDATE clients c
SET contact_name  = btrim(ct.firstname || ' ' || ct.lastname),
    contact_role  = ct.role,
    contact_email = ct.email
FROM contacts ct
WHERE ct.id = c.primary_contact_id;

ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_primary_contact_fk;

ALTER TABLE clients
    DROP COLUMN IF EXISTS primary_contact_id;

DROP TABLE IF EXISTS contacts;
