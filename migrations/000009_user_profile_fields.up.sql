-- Etat civil et coordonnees d'un compte.
--
-- Six colonnes que l'ecran « Paramètres » demande : le sexe, le telephone et
-- l'adresse postale decoupee. Elles vivent sur le compte et non dans une table
-- de profil a part — c'est une relation un a un que personne ne lit sans lire
-- le compte, et la separer couterait une jointure sur chaque session.
--
-- Toutes NOT NULL avec defaut vide, comme `description` sur les projets : un
-- compte sans adresse est un cas normal, pas une absence de valeur. Cela evite
-- un `*string` et un test de nil a chaque lecture.
--
-- `gender` suit le parti de `priority` : un CHECK plutot qu'une table de
-- reference. La chaine vide y est admise, c'est ce que vaut « non renseigne » —
-- un compte n'a pas a declarer son sexe pour exister.
ALTER TABLE users
    ADD COLUMN gender      text NOT NULL DEFAULT '',
    ADD COLUMN phone       text NOT NULL DEFAULT '',
    ADD COLUMN address     text NOT NULL DEFAULT '',
    ADD COLUMN postal_code text NOT NULL DEFAULT '',
    ADD COLUMN city        text NOT NULL DEFAULT '',
    ADD COLUMN country     text NOT NULL DEFAULT '';

ALTER TABLE users
    ADD CONSTRAINT users_gender_check
        CHECK (gender IN ('', 'male', 'female', 'nonbinary'));
