-- Contacts du CRM.
--
-- Jusqu'ici un client portait son interlocuteur en trois colonnes plates. Cela
-- suffisait pour afficher « qui repond au telephone », pas pour un module qui
-- doit lister les personnes, les chercher et en tenir plusieurs par client.
--
-- Le contact appartient a son client : une personne est l'interlocuteur d'une
-- entreprise, et la meme adresse chez deux clients designe deux relations
-- distinctes. D'ou `client_id` NOT NULL et la suppression en cascade — un
-- contact sans entreprise n'a personne a representer.
CREATE TABLE contacts (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id  uuid        NOT NULL REFERENCES clients (id) ON DELETE CASCADE,
    firstname  text        NOT NULL DEFAULT '',
    lastname   text        NOT NULL DEFAULT '',
    role       text        NOT NULL DEFAULT '',
    email      citext,
    phone      text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    -- Prenom et nom sont separes comme sur les comptes, et l'un des deux
    -- suffit : on connait souvent « Camille » avant de connaitre son nom.
    CONSTRAINT contacts_name_not_blank CHECK (btrim(firstname) <> '' OR btrim(lastname) <> '')
);

CREATE INDEX contacts_client_idx ON contacts (client_id) WHERE deleted_at IS NULL;
-- Recherche floue du menu deroulant qui designe le contact principal.
CREATE INDEX contacts_name_trgm_idx ON contacts USING gin ((firstname || ' ' || lastname) gin_trgm_ops);

-- Cible de la cle etrangere composite ci-dessous. `id` etant deja la cle
-- primaire, cet index unique ne contraint rien de neuf : il existe pour que
-- (client_id, id) soit referencable.
ALTER TABLE contacts
    ADD CONSTRAINT contacts_client_id_unique UNIQUE (client_id, id);

-- Contact principal du client : celui que les ecrans montrent quand ils n'en
-- montrent qu'un.
ALTER TABLE clients
    ADD COLUMN primary_contact_id uuid;

-- Cle etrangere composite plutot que simple : elle interdit en base qu'un
-- client designe le contact d'un autre client. Un CHECK ne saurait pas le
-- faire — il ne peut pas interroger une autre table — et un declencheur le
-- ferait au prix d'une fonction a maintenir.
--
-- `SET NULL (primary_contact_id)` ne vide que cette colonne : sans la liste,
-- Postgres tenterait de mettre `id` a NULL aussi, ce que la cle primaire
-- refuse. La forme est disponible depuis Postgres 15.
ALTER TABLE clients
    ADD CONSTRAINT clients_primary_contact_fk
        FOREIGN KEY (id, primary_contact_id) REFERENCES contacts (client_id, id)
        ON DELETE SET NULL (primary_contact_id);

-- Reprise des interlocuteurs deja saisis. Le nom complet se coupe au premier
-- espace ; sans espace, tout part dans le prenom plutot que d'etre recopie des
-- deux cotes.
WITH repris AS (
    INSERT INTO contacts (client_id, firstname, lastname, role, email)
    SELECT
        id,
        CASE
            WHEN position(' ' IN btrim(contact_name)) = 0 THEN btrim(contact_name)
            ELSE split_part(btrim(contact_name), ' ', 1)
        END,
        CASE
            WHEN position(' ' IN btrim(contact_name)) = 0 THEN ''
            ELSE btrim(substring(btrim(contact_name) FROM position(' ' IN btrim(contact_name)) + 1))
        END,
        contact_role,
        contact_email
    FROM clients
    WHERE btrim(contact_name) <> ''
    RETURNING id AS contact_id, client_id
)
UPDATE clients c
SET primary_contact_id = r.contact_id
FROM repris r
WHERE c.id = r.client_id;

-- Les trois colonnes ont fini leur service : les garder ferait deux sources de
-- verite pour le meme interlocuteur, qui divergeraient a la premiere
-- modification faite d'un seul cote.
ALTER TABLE clients
    DROP COLUMN contact_name,
    DROP COLUMN contact_role,
    DROP COLUMN contact_email;
