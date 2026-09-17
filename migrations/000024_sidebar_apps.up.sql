-- Applications jointes depuis le rail.
--
-- Elles etaient ecrites en dur dans le front, avec leur logo en fichier local :
-- ajouter un outil demandait de toucher au code et de redeployer. La liste
-- devient une donnee, tenue depuis les parametres.
CREATE TABLE sidebar_apps (
    id   uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name text NOT NULL,
    url  text NOT NULL,

    -- Cle de stockage du logo, nulle tant qu'aucun fichier n'a ete depose. Le
    -- magasin est celui des avatars et des pieces jointes : un seul endroit ou
    -- les octets vivent.
    logo_key text,

    -- Teinte de la pastille affichee a defaut de logo, a l'initiale du nom.
    -- Le rail ne se troue jamais : une app sans logo reste cliquable.
    color text NOT NULL DEFAULT '#73757c',

    -- Rang dans le rail. Un entier libre plutot qu'une contrainte d'unicite :
    -- reordonner deux lignes qui echangeraient leur rang buterait sinon sur
    -- l'index le temps de l'echange.
    position integer NOT NULL DEFAULT 0,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    CONSTRAINT sidebar_apps_name_check CHECK (name <> ''),
    CONSTRAINT sidebar_apps_url_check  CHECK (url <> '')
);

-- Le rail les lit dans l'ordre, a chaque page : l'index porte les deux colonnes
-- du tri pour que la lecture se fasse sans tri.
CREATE INDEX sidebar_apps_position_idx
    ON sidebar_apps (position, name) WHERE deleted_at IS NULL;

-- Reprise des quatre outils que le rail affichait en dur. Leurs logos sont des
-- fichiers du depot front, que le magasin ne connait pas : elles s'affichent en
-- pastille jusqu'a ce qu'on depose leur image.
INSERT INTO sidebar_apps (name, url, color, position) VALUES
    ('Google Drive', 'https://drive.google.com',   '#1a73e8', 1),
    ('Coolify',      'https://cool.plugiit.com',   '#8b5cf6', 2),
    ('Uptime Kuma',  'https://uptime.plugiit.com', '#059669', 3),
    ('Proxmox',      'https://prox.plugiit.com',   '#e57000', 4);
