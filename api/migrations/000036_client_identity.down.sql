DROP INDEX IF EXISTS clients_siret_idx;

ALTER TABLE clients
    DROP CONSTRAINT IF EXISTS clients_kind_check,
    DROP COLUMN IF EXISTS registry_checked_at,
    DROP COLUMN IF EXISTS legal_form,
    DROP COLUMN IF EXISTS legal_name,
    DROP COLUMN IF EXISTS kind;
