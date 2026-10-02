DROP TABLE IF EXISTS email_outbox;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS invitations;
ALTER TABLE users DROP COLUMN IF EXISTS disabled_at;
