-- Retour au CHECK statique : la liste des roles redevient figee dans le schema.
ALTER TABLE users DROP CONSTRAINT users_role_fkey;

ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('admin', 'team', 'client'));

DROP TABLE role_permissions;
DROP TABLE permissions;
DROP TABLE roles;
