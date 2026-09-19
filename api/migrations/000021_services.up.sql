-- Services.
--
-- Referentiel des prestations de l'agence : developpement, design, SEO,
-- maintenance. Une nomenclature, pas une grille tarifaire — ni taux ni unite
-- de vente, qui appelleraient une remise puis une ligne puis un devis, quand
-- la facturation reste hors de ce projet.
CREATE TABLE services (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    -- Teinte de la pastille, comme les statuts des projets et des tickets.
    color       text NOT NULL DEFAULT '#73757c',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,

    CONSTRAINT services_name_check CHECK (name <> '')
);

-- Deux services du meme nom se confondraient dans un menu deroulant. Unicite
-- sur les seuls vivants, comme pour les clients : un service supprime ne doit
-- pas empecher d'en recreer un du meme nom.
CREATE UNIQUE INDEX services_name_unique
    ON services (lower(name)) WHERE deleted_at IS NULL;

-- La liste s'affiche par ordre alphabetique : c'est un referentiel qu'on
-- parcourt, pas un flux qu'on lit du plus recent.
CREATE INDEX services_name_idx
    ON services (name) WHERE deleted_at IS NULL;
