-- Comptes de portail rattaches a un client, et agregats de la liste CRM.
--
-- Le rattachement va du compte vers le client, et non l'inverse : une
-- entreprise cliente a souvent plusieurs interlocuteurs, et chacun doit
-- pouvoir se connecter sous son propre nom. Poser `user_id` sur `clients`
-- aurait impose un compte unique par entreprise, donc des identifiants
-- partages — exactement ce qu'un portail client ne doit pas demander.
ALTER TABLE users
    ADD COLUMN client_id uuid REFERENCES clients (id) ON DELETE RESTRICT;

-- Seul un compte de role `client` appartient a un client. La reciproque n'est
-- pas imposee : un compte client tout juste cree peut attendre son
-- rattachement, et bloquer l'insertion obligerait a tout ecrire en une seule
-- requete.
ALTER TABLE users
    ADD CONSTRAINT users_client_id_role_check
        CHECK (client_id IS NULL OR role = 'client');

-- Le portail lit « les projets de mon client » a chaque requete : sans cet
-- index, chaque page du portail balaie la table des comptes.
CREATE INDEX users_client_idx ON users (client_id) WHERE deleted_at IS NULL;

-- Agregats de la liste CRM, tenus par declencheur comme `tasks_total` sur les
-- projets. La liste des clients ne fait donc aucun COUNT.
ALTER TABLE clients
    ADD COLUMN projects_active integer NOT NULL DEFAULT 0,
    ADD COLUMN portal_users    integer NOT NULL DEFAULT 0;

ALTER TABLE clients
    ADD CONSTRAINT clients_counts_positive
        CHECK (projects_active >= 0 AND portal_users >= 0);

-- Un projet est actif tant qu'il n'est pas livre : c'est ce que la colonne
-- « Projets actifs » annonce a l'ecran. Un projet livre reste rattache a son
-- client, il ne compte simplement plus.
CREATE OR REPLACE FUNCTION refresh_client_projects_active(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE clients c
    SET projects_active = (
        SELECT count(*)
        FROM projects p
        WHERE p.client_id = target
          AND p.deleted_at IS NULL
          AND p.status <> 'livre'
    )
    WHERE c.id = target;
$$;

CREATE OR REPLACE FUNCTION refresh_client_portal_users(target uuid)
RETURNS void
LANGUAGE sql
AS $$
    UPDATE clients c
    SET portal_users = (
        SELECT count(*)
        FROM users u
        WHERE u.client_id = target
          AND u.deleted_at IS NULL
    )
    WHERE c.id = target;
$$;

CREATE OR REPLACE FUNCTION projects_touch_client_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM refresh_client_projects_active(OLD.client_id);
        RETURN OLD;
    END IF;

    PERFORM refresh_client_projects_active(NEW.client_id);

    -- Un projet revendu a un autre client laisse un compteur a corriger
    -- derriere lui.
    IF TG_OP = 'UPDATE' AND NEW.client_id <> OLD.client_id THEN
        PERFORM refresh_client_projects_active(OLD.client_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER projects_client_counts
AFTER INSERT OR UPDATE OF status, client_id, deleted_at OR DELETE ON projects
FOR EACH ROW EXECUTE FUNCTION projects_touch_client_counts();

-- `client_id` est nullable des deux cotes : un compte de l'agence n'appartient
-- a aucun client, et rattacher ou detacher un compte doit corriger les deux
-- lignes concernees. D'ou les tests de NULL, absents du declencheur des
-- projets ou la colonne est NOT NULL.
CREATE OR REPLACE FUNCTION users_touch_client_counts()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD.client_id IS NOT NULL THEN
            PERFORM refresh_client_portal_users(OLD.client_id);
        END IF;

        RETURN OLD;
    END IF;

    IF NEW.client_id IS NOT NULL THEN
        PERFORM refresh_client_portal_users(NEW.client_id);
    END IF;

    IF TG_OP = 'UPDATE'
       AND OLD.client_id IS NOT NULL
       AND OLD.client_id IS DISTINCT FROM NEW.client_id
    THEN
        PERFORM refresh_client_portal_users(OLD.client_id);
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER users_client_counts
AFTER INSERT OR UPDATE OF client_id, deleted_at OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION users_touch_client_counts();

-- Les projets existent deja : sans ce rattrapage, la colonne resterait a zero
-- jusqu'a la prochaine ecriture sur chaque projet.
UPDATE clients c
SET projects_active = (
    SELECT count(*)
    FROM projects p
    WHERE p.client_id = c.id
      AND p.deleted_at IS NULL
      AND p.status <> 'livre'
);
