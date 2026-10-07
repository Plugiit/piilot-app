-- Logo d'un projet : une image par projet, deposee depuis ses parametres.
--
-- La colonne porte la cle du magasin de fichiers, pas une adresse : c'est le
-- code qui sait sous quel chemin un logo se relit. Nulle sans logo.
ALTER TABLE projects ADD COLUMN logo_key text;
