-- Pipeline commercial, chargé de compte, coordonnees d'entreprise, et unicite
-- des contacts.

-- Etape du cycle de vente. Un CHECK plutot qu'une table de reference, comme
-- `status` sur les projets : cinq valeurs figees que le front connait par
-- coeur valent mieux qu'une jointure sur chaque ligne pour lire « Actif ».
--
-- L'ordre du kanban est celui de la liste, et il vit dans le front : la base
-- n'a pas a savoir que « devis » vient apres « lead ».
--
-- Reserve assumee : « devis » ne pointe vers aucun devis. La facturation reste
-- sur l'ancienne plateforme, et cette etape ne vaut donc que ce que l'equipe y
-- met a la main.
ALTER TABLE clients
    ADD COLUMN status text NOT NULL DEFAULT 'lead';

ALTER TABLE clients
    ADD CONSTRAINT clients_status_check
        CHECK (status IN ('lead', 'devis', 'actif', 'veille', 'perdu'));

-- Chargé de compte : qui suit ce client dans l'agence.
--
-- `SET NULL` au depart d'un compte : un client ne disparait pas parce que la
-- personne qui le suivait a quitte l'agence — il redevient simplement sans
-- referent, ce qui se voit et se corrige.
--
-- Qu'il s'agisse d'un compte interne n'est pas impose ici : un CHECK ne sait
-- pas interroger une autre table, et le service s'en charge. La colonne accepte
-- donc techniquement un compte client, que rien dans l'interface ne propose.
ALTER TABLE clients
    ADD COLUMN account_manager_id uuid REFERENCES users (id) ON DELETE SET NULL;

-- Coordonnees de l'entreprise. Toutes NOT NULL avec defaut vide, comme celles
-- du profil utilisateur en 000009 : une adresse absente est un cas normal, pas
-- une valeur manquante, et cela evite un `*string` et un test de nil partout.
--
-- `siret` et `vat_number` existent aussi sur l'ancienne plateforme, qui porte
-- la facturation : ils seront donc saisis a deux endroits tant que les deux
-- coexistent.
ALTER TABLE clients
    ADD COLUMN website     text NOT NULL DEFAULT '',
    ADD COLUMN phone       text NOT NULL DEFAULT '',
    ADD COLUMN address     text NOT NULL DEFAULT '',
    ADD COLUMN postal_code text NOT NULL DEFAULT '',
    ADD COLUMN city        text NOT NULL DEFAULT '',
    ADD COLUMN country     text NOT NULL DEFAULT '',
    ADD COLUMN siret       text NOT NULL DEFAULT '',
    ADD COLUMN vat_number  text NOT NULL DEFAULT '';

-- Le kanban lit une colonne a la fois ; sans cet index il balaie la table pour
-- chacune des cinq.
CREATE INDEX clients_status_idx ON clients (status) WHERE deleted_at IS NULL;
CREATE INDEX clients_account_manager_idx ON clients (account_manager_id) WHERE deleted_at IS NULL;

-- Les clients qui portent deja des projets ne sont pas des pistes : les laisser
-- a « lead » obligerait a les reclasser un par un le lendemain de la migration.
UPDATE clients c
SET status = 'actif'
WHERE c.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.client_id = c.id AND p.deleted_at IS NULL);

-- Unicite de l'adresse par client : la meme personne ne s'inscrit pas deux fois
-- chez le meme client. L'index est partiel des deux cotes — un contact sans
-- adresse ne contraint rien, un contact efface ne bloque pas sa recreation.
--
-- Deux homonymes sans e-mail restent possibles : rien ne permet de dire qu'il
-- s'agit de la meme personne, et refuser serait refuser un cas legitime.
CREATE UNIQUE INDEX contacts_email_unique
    ON contacts (client_id, lower(email))
    WHERE email IS NOT NULL AND deleted_at IS NULL;
