-- Ce que la fiche projet affiche et que le schema ne portait pas encore :
-- priorite des taches, lien externe, pieces jointes, mise en favori.

-- Priorite d'une tache.
--
-- Meme parti que celle du projet : un CHECK plutot qu'une table de reference,
-- trois valeurs que le front connait par coeur. Defaut 'medium' pour que les
-- taches deja en base aient une valeur sans qu'on ait a en inventer une.
ALTER TABLE tasks
    ADD COLUMN priority text NOT NULL DEFAULT 'medium';

ALTER TABLE tasks
    ADD CONSTRAINT tasks_priority_check CHECK (priority IN ('low', 'medium', 'high'));

-- Lien de travail du projet — maquette Figma, dossier partage, ce que l'equipe
-- ouvre en premier. Une seule adresse et non une table de liens : la fiche n'en
-- affiche qu'une, et une table appellerait un ecran pour la gerer.
--
-- NOT NULL avec defaut vide : un projet sans lien est un cas normal.
ALTER TABLE projects
    ADD COLUMN external_url text NOT NULL DEFAULT '';

-- Pieces jointes d'un projet.
--
-- `storage_key` est le nom sous lequel le fichier est ecrit sur le disque : un
-- identifiant tire au sort, jamais le nom donne par l'utilisateur. C'est ce qui
-- ferme la traversee de repertoire — un fichier appele « ../../etc/passwd » ne
-- peut ecrire qu'a l'emplacement que la base lui a attribue.
--
-- `filename` ne sert qu'a l'affichage et au telechargement.
CREATE TABLE project_files (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id   uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    filename     text        NOT NULL,
    content_type text        NOT NULL,
    size_bytes   bigint      NOT NULL,
    storage_key  text        NOT NULL UNIQUE,
    uploaded_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT project_files_filename_not_blank CHECK (btrim(filename) <> ''),
    CONSTRAINT project_files_size_positive CHECK (size_bytes > 0)
);

CREATE INDEX project_files_project_idx ON project_files (project_id);

-- Projets mis en favori, par compte.
--
-- L'etoile est un marquage personnel, pas un attribut du projet : deux chefs de
-- projet n'ont pas les memes. D'ou la table d'association plutot qu'une colonne
-- booleenne sur `projects`.
CREATE TABLE project_favorites (
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    project_id uuid        NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, project_id)
);
