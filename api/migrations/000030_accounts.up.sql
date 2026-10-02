-- Gestion des comptes depuis l'interface : desactivation, invitations,
-- mot de passe oublie, et la file d'envoi des e-mails qui les porte.

-- Un compte desactive ne se connecte plus et perd ses sessions. Distinct de la
-- suppression : il garde son historique (taches, temps saisi, commentaires) et
-- se reactive d'un clic.
ALTER TABLE users ADD COLUMN disabled_at timestamptz;

-- Invitations. Le compte n'existe qu'a l'acceptation : c'est l'invite qui
-- choisit son mot de passe, l'admin n'en voit jamais aucun.
--
-- Le jeton n'est stocke que hache, comme les jetons de session : une fuite de
-- la table ne permet pas d'accepter une invitation a la place de son
-- destinataire.
CREATE TABLE invitations (
    id          uuid        PRIMARY KEY DEFAULT uuid_generate_v4(),
    email       citext      NOT NULL,
    firstname   text        NOT NULL DEFAULT '',
    lastname    text        NOT NULL DEFAULT '',
    role        text        NOT NULL REFERENCES roles (code) ON UPDATE CASCADE,
    client_id   uuid        REFERENCES clients (id) ON DELETE CASCADE,
    token_hash  bytea       NOT NULL,
    invited_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    expires_at  timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at  timestamptz,
    user_id     uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    -- Meme regle que pour les comptes : un client designe, et seulement pour
    -- le role client.
    CONSTRAINT invitations_client_role_check
        CHECK ((role = 'client') = (client_id IS NOT NULL))
);

CREATE UNIQUE INDEX invitations_token_unique ON invitations (token_hash);
-- Une seule invitation ouverte par adresse : en renvoyer une remplace le
-- jeton de la precedente plutot que d'en empiler deux valides.
CREATE UNIQUE INDEX invitations_open_email_unique
    ON invitations (email) WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- Reinitialisations de mot de passe. Jeton hache, a usage unique, de courte
-- duree.
CREATE TABLE password_resets (
    id         uuid        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX password_resets_token_unique ON password_resets (token_hash);
CREATE INDEX password_resets_user_idx ON password_resets (user_id) WHERE used_at IS NULL;

-- File d'envoi des e-mails. Un e-mail n'est jamais envoye pendant la requete
-- qui le provoque : il est ecrit ici, et une tache de fond l'envoie, avec de
-- nouvelles tentatives si le serveur SMTP ne repond pas.
--
-- Les corps portent des liens a usage unique : ils sont effaces une fois
-- l'e-mail parti, pour que la table ne garde pas de jeton valide en clair.
CREATE TABLE email_outbox (
    id              uuid        PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind            text        NOT NULL,
    to_address      text        NOT NULL,
    subject         text        NOT NULL,
    text_body       text        NOT NULL,
    html_body       text        NOT NULL,
    status          text        NOT NULL DEFAULT 'pending',
    attempts        integer     NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    last_error      text        NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,

    CONSTRAINT email_outbox_status_check CHECK (status IN ('pending', 'sent', 'failed'))
);

CREATE INDEX email_outbox_pending_idx ON email_outbox (next_attempt_at) WHERE status = 'pending';

-- Libelles affiches tels quels par l'ecran « Roles » : ils prennent leurs
-- accents, ecrits sans a l'epoque ou ils ne sortaient pas de la base.
UPDATE permissions SET label = CASE code
    WHEN 'clients.write'     THEN 'Créer et modifier les clients'
    WHEN 'deliverables.write' THEN 'Créer et modifier les livrables'
    WHEN 'projects.write'    THEN 'Créer et modifier les projets'
    WHEN 'roles.read'        THEN 'Consulter les rôles et permissions'
    WHEN 'roles.write'       THEN 'Modifier les rôles et permissions'
    WHEN 'system.update'     THEN 'Mettre à jour l''application'
    WHEN 'tasks.read'        THEN 'Consulter les tâches'
    WHEN 'tasks.write'       THEN 'Créer et modifier les tâches'
    WHEN 'tickets.write'     THEN 'Créer et commenter les tickets'
    WHEN 'time.read'         THEN 'Consulter le temps passé'
    WHEN 'time.write'        THEN 'Saisir du temps passé'
    WHEN 'users.write'       THEN 'Inviter et gérer les comptes'
    ELSE label END;

UPDATE roles SET label = 'Équipe' WHERE code = 'team';
