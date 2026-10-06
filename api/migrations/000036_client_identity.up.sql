-- Identite d'un client : particulier ou professionnel.
--
-- Un particulier est une personne : son nom est celui de son contact
-- principal, et il n'a ni SIRET ni raison sociale. Un professionnel est une
-- entreprise, identifiee par son SIRET, dont la raison sociale et la forme
-- juridique se relisent au registre.
--
-- Les clients deja inscrits sont des entreprises : c'est ce qu'une agence
-- sert, et ce que la table supposait jusqu'ici.
ALTER TABLE clients
    ADD COLUMN kind text NOT NULL DEFAULT 'professionnel',
    -- Raison sociale telle que declaree au registre. `name` reste le nom
    -- d'usage, celui que l'agence emploie et que les ecrans affichent.
    ADD COLUMN legal_name text NOT NULL DEFAULT '',
    -- Forme juridique, en clair : « SAS, société par actions simplifiée ».
    ADD COLUMN legal_form text NOT NULL DEFAULT '',
    -- Date a laquelle le SIRET a ete retrouve au registre, nulle tant qu'il
    -- ne l'a pas ete. Elle tombe des que le SIRET change sans nouvelle
    -- verification : elle ne vaut que pour le numero verifie.
    ADD COLUMN registry_checked_at timestamptz;

ALTER TABLE clients
    ADD CONSTRAINT clients_kind_check CHECK (kind IN ('particulier', 'professionnel'));

-- Le doublon se cherche par SIRET a chaque creation d'un professionnel.
CREATE INDEX clients_siret_idx ON clients (siret) WHERE deleted_at IS NULL AND siret <> '';
