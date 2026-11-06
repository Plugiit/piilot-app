-- Le bot Discord de l'agence : un acces machine a l'API, la liaison d'un
-- compte Discord par code a usage unique, et un flux d'evenements que le bot
-- relit.
--
-- L'API n'appelle jamais Discord : le bot lui transmet ce que le membre a tape,
-- et relit le flux pour attribuer ou retirer ses roles.

-- Role machine. Systeme, et jamais porte par un compte humain : une cle n'a
-- pas de mot de passe, un compte n'a pas les droits du bot.
INSERT INTO roles (code, label, is_system) VALUES ('bot', 'Bot', true);

ALTER TABLE users ADD CONSTRAINT users_role_not_bot CHECK (role <> 'bot');
ALTER TABLE invitations ADD CONSTRAINT invitations_role_not_bot CHECK (role <> 'bot');

INSERT INTO permissions (code, label) VALUES
    ('discord.link',   'Lier son compte Discord'),
    ('discord.read',   'Lire les liaisons Discord'),
    ('discord.redeem', 'Racheter un code de liaison Discord'),
    ('events.read',    'Lire le flux d''événements');

-- discord.link : chacun pour son propre compte.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code IN ('admin', 'team', 'client') AND p.code = 'discord.link';

-- Le reste au seul bot. Ces permissions ne s'exercent que sous /api/v1/bot,
-- ou aucune session n'entre : les donner a un role de compte n'ouvrirait rien.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'bot' AND p.code IN ('discord.read', 'discord.redeem', 'events.read');

-- Les cles du bot. Seule l'empreinte est gardee : la cle n'est montree qu'a sa
-- creation. Plusieurs cles actives a la fois, pour tourner sans coupure.
CREATE TABLE bot_keys (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name         text NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9._-]{0,63}$'),
    key_hash     bytea NOT NULL,
    role         text NOT NULL DEFAULT 'bot' REFERENCES roles (code) CHECK (role = 'bot'),
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);
CREATE UNIQUE INDEX bot_keys_hash_unique ON bot_keys (key_hash);
CREATE UNIQUE INDEX bot_keys_name_active ON bot_keys (name) WHERE revoked_at IS NULL;

-- Une liaison par compte, un compte Discord lie a un seul compte au plus.
-- L'identifiant Discord est une donnee personnelle : il part avec le compte.
CREATE TABLE discord_links (
    user_id         uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    discord_user_id text NOT NULL CHECK (discord_user_id ~ '^[0-9]{17,20}$'),
    linked_at       timestamptz NOT NULL DEFAULT now()
);
-- Sert aussi la liste du bot, parcourue dans cet ordre.
CREATE UNIQUE INDEX discord_links_discord_user_unique ON discord_links (discord_user_id);

-- Le code en cours d'un compte : un seul, le suivant remplace le precedent.
-- HMAC du code et non le code : 40 bits se retrouveraient en quelques minutes
-- depuis une simple empreinte SHA-256.
CREATE TABLE discord_link_codes (
    user_id    uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    code_hash  bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX discord_link_codes_hash_unique ON discord_link_codes (code_hash);
CREATE INDEX discord_link_codes_expires_idx ON discord_link_codes (expires_at);

-- Le flux que le bot relit, ecrit dans la transaction du changement. Generique :
-- un type et des donnees minimales. Les types sont listes ici, comme ceux des
-- notifications, pour que le contrat se lise dans le schema.
CREATE TABLE bot_events (
    id         bigint PRIMARY KEY,
    type       text NOT NULL CHECK (type IN ('discord_link.upserted', 'discord_link.removed')),
    data       jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bot_events_created_idx ON bot_events (created_at);

-- Le compteur des evenements. Chaque emission l'incremente dans sa transaction :
-- le verrou de la ligne, tenu jusqu'au commit, fait que les identifiants
-- deviennent visibles dans l'ordre. Une sequence ne le garantit pas — 11 peut
-- etre valide avant 10, et un lecteur deja passe a 11 ne verrait jamais 10.
--
-- purged_through : tout evenement d'identifiant inferieur ou egal a ete purge.
-- Un curseur plus ancien a perdu des evenements.
CREATE TABLE bot_event_cursor (
    id             boolean PRIMARY KEY DEFAULT true CHECK (id),
    last_id        bigint NOT NULL DEFAULT 0,
    purged_through bigint NOT NULL DEFAULT 0
);
INSERT INTO bot_event_cursor DEFAULT VALUES;
