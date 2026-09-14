-- Contacts libres : une personne peut exister avant son entreprise.
--
-- La migration precedente liait tout contact a un client. C'etait trop strict :
-- on rencontre quelqu'un avant de savoir chez qui il travaille, et on inscrit
-- un interlocuteur pressenti avant que le client ne soit ouvert. Un contact
-- naît donc libre ou chez un client, et se rattache ensuite.
--
-- Ce que la nullabilite ne change pas :
--
--   * la cle etrangere composite `clients (id, primary_contact_id)` ->
--     `contacts (client_id, id)` continue d'exiger que le contact principal
--     appartienne au client. Un contact libre ne peut donc pas etre designe
--     principal — il faut d'abord le rattacher, ce que fait la creation d'un
--     client qui le choisit.
--   * la suppression en cascade ne concerne que les rattaches : un contact
--     libre n'a aucun client dont la disparition l'emporterait.
ALTER TABLE contacts
    ALTER COLUMN client_id DROP NOT NULL;

-- L'index partiel existant ne couvre que les rattaches. Celui-ci sert la liste
-- des contacts libres, que le formulaire de creation d'un client deroule.
CREATE INDEX contacts_free_idx ON contacts (lastname, firstname)
    WHERE client_id IS NULL AND deleted_at IS NULL;
