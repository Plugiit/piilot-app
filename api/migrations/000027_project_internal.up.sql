-- Projet interne : le travail de l'agence pour elle-meme (site, outils,
-- formation, prospection). Le temps qu'on y pointe n'est pas facturable.
--
-- Le caractere facturable d'une heure n'est pas stocke sur la saisie : il se
-- deduit du projet. Basculer un projet en interne reclasse donc tout son temps,
-- passe compris — c'est voulu, une erreur de classement se corrige a un seul
-- endroit.
ALTER TABLE projects
    ADD COLUMN is_internal boolean NOT NULL DEFAULT false;
