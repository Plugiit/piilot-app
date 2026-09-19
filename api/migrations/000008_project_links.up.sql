-- Trois liens de travail plutot qu'un.
--
-- `external_url` ne disait pas de quoi il s'agissait : l'ecran affichait une
-- adresse sans pouvoir annoncer « voici la preproduction ». Trois colonnes
-- nommees le permettent, et la page de parametres a trois champs etiquetes
-- plutot qu'un champ generique.
--
-- Trois colonnes et non une table de liens : l'agence en veut exactement
-- trois, et une table aurait impose un ecran pour les gerer.
ALTER TABLE projects RENAME COLUMN external_url TO figma_url;

ALTER TABLE projects
    ADD COLUMN prod_url    text NOT NULL DEFAULT '',
    ADD COLUMN preprod_url text NOT NULL DEFAULT '';
