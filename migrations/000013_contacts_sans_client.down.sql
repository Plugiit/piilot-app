DROP INDEX IF EXISTS contacts_free_idx;

-- Les contacts libres n'ont pas de client ou retourner : la colonne redevenant
-- obligatoire, ils sont supprimes. C'est la perte assumee du retour en arriere.
DELETE FROM contacts WHERE client_id IS NULL;

ALTER TABLE contacts
    ALTER COLUMN client_id SET NOT NULL;
