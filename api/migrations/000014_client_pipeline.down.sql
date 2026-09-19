DROP INDEX IF EXISTS contacts_email_unique;
DROP INDEX IF EXISTS clients_account_manager_idx;
DROP INDEX IF EXISTS clients_status_idx;

ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_status_check;

ALTER TABLE clients
    DROP COLUMN IF EXISTS vat_number,
    DROP COLUMN IF EXISTS siret,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS postal_code,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS website,
    DROP COLUMN IF EXISTS account_manager_id,
    DROP COLUMN IF EXISTS status;
