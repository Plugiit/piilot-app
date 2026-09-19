-- Recuperation automatique du logo depuis l'adresse de l'app.
--
-- Le favicon d'un outil est la bonne valeur par defaut : on le reconnait, et
-- personne n'a envie de deposer quatre fichiers a la main. Il ne remplace pas
-- le depot — un favicon fait souvent 16 ou 32 pixels — il evite d'avoir a s'en
-- occuper.
--
-- La recuperation se fait en tache de fond et jamais dans le cycle d'une
-- requete : aller chercher une adresse tierce pendant qu'un ecran attend, c'est
-- un ecran qui attend un serveur qu'on ne maitrise pas.

-- Vrai quand le logo vient du favicon : une nouvelle tentative peut alors
-- l'ecraser. Un logo depose a la main ne se fait jamais remplacer par une
-- recuperation automatique.
ALTER TABLE sidebar_apps
    ADD COLUMN logo_is_favicon boolean NOT NULL DEFAULT false;

-- Date de la derniere tentative, quelle qu'en soit l'issue. Nulle tant qu'on
-- n'a pas essaye : c'est ce qui designe le travail restant.
--
-- Une date plutot qu'un drapeau : un echec ne doit pas etre retente en boucle,
-- et l'anciennete dit quand reessayer.
ALTER TABLE sidebar_apps
    ADD COLUMN favicon_attempted_at timestamptz;

-- Le job cherche les apps sans logo qu'il n'a jamais essayees, ou essayees il y
-- a longtemps. L'index partiel ne porte que celles-la : les apps pourvues d'un
-- logo depose n'ont rien a faire dans ce balayage.
CREATE INDEX sidebar_apps_favicon_todo_idx
    ON sidebar_apps (favicon_attempted_at NULLS FIRST)
    WHERE deleted_at IS NULL AND logo_key IS NULL;
