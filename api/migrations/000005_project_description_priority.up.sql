-- Description et priorite d'un projet.
--
-- Deux colonnes que la liste affiche sur chaque carte : le resume sous le titre
-- et le drapeau de priorite a cote de l'echeance. Elles arrivent apres coup
-- parce que la maquette de la liste les a demandees ; la table les porte en
-- propre plutot que de les faire deviner par l'ecran.
--
-- `description` est NOT NULL avec defaut vide : un projet sans resume est un cas
-- normal, pas une absence de valeur. Cela evite un `*string` et un test de nil
-- a chaque lecture.
--
-- `priority` suit le meme parti que `status` : un CHECK plutot qu'une table de
-- reference. Trois valeurs figees que le front connait par coeur, et aucune
-- jointure ajoutee sur une liste qui en fait deja une.
ALTER TABLE projects
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN priority    text NOT NULL DEFAULT 'medium';

ALTER TABLE projects
    ADD CONSTRAINT projects_priority_check CHECK (priority IN ('low', 'medium', 'high'));
