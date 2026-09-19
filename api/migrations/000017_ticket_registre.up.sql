-- Registre d'un ticket : ce qui s'y dit, et ce qui lui arrive.
--
-- Deux tables et non une seule a discriminant : un message porte un texte et un
-- destinataire, un evenement porte un champ et deux valeurs. Les loger ensemble
-- imposerait une moitie de colonnes nulles sur chaque ligne, et un CHECK pour
-- dire lesquelles selon le cas. L'ecran les entrelace a la lecture, ce que le
-- service fait en fusionnant deux listes triees.

-- Ce qui s'y dit.
CREATE TABLE ticket_messages (
    id        uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id uuid NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,

    -- `SET NULL` au depart d'un compte : le message reste au registre, il perd
    -- seulement son auteur. Effacer la trace parce que la personne est partie
    -- rendrait l'historique faux.
    author_id uuid REFERENCES users (id) ON DELETE SET NULL,

    body text NOT NULL,

    -- Note interne : le portail client ne la sert jamais.
    --
    -- C'est la colonne la plus sensible de la table. Un ticket d'agence a deux
    -- publics, et se tromper de destinataire ne se rattrape pas : le defaut est
    -- donc `false`, pour qu'une note interne soit toujours un choix explicite
    -- et jamais le resultat d'un champ oublie.
    is_internal boolean NOT NULL DEFAULT false,

    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,

    CONSTRAINT ticket_messages_body_not_blank CHECK (btrim(body) <> '')
);

-- Le registre se lit du plus ancien au plus recent, ticket par ticket.
CREATE INDEX ticket_messages_ticket_idx
    ON ticket_messages (ticket_id, created_at)
    WHERE deleted_at IS NULL;

-- Ce qui lui arrive.
--
-- Pas de suppression logique ici : un evenement est un fait date, on ne le
-- retire pas du registre. C'est ce qui separe le journal du message.
CREATE TABLE ticket_events (
    id        uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id uuid NOT NULL REFERENCES tickets (id) ON DELETE CASCADE,
    actor_id  uuid REFERENCES users (id) ON DELETE SET NULL,

    -- Ce qui a change, et entre quelles valeurs.
    --
    -- Les valeurs sont du texte libre et non des cles etrangeres : elles
    -- figent ce qui etait vrai ce jour-la. Un statut retire de la nomenclature
    -- doit continuer de s'afficher dans les registres qui l'ont connu.
    field     text NOT NULL,
    old_value text NOT NULL DEFAULT '',
    new_value text NOT NULL DEFAULT '',

    created_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ticket_events_field_check
        CHECK (field IN ('status', 'priority', 'tracker', 'assignee'))
);

CREATE INDEX ticket_events_ticket_idx ON ticket_events (ticket_id, created_at);
