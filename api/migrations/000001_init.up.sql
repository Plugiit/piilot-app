-- Extensions utilisees par le schema :
--   citext    : emails insensibles a la casse, sans LOWER() dans chaque requete
--   pg_trgm   : recherche floue sur les libelles (index GIN)
--   uuid-ossp : generation d'UUID cote base pour les inserts sans applicatif
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         citext      NOT NULL,
    password_hash text        NOT NULL,
    firstname     text        NOT NULL DEFAULT '',
    lastname      text        NOT NULL DEFAULT '',
    role          text        NOT NULL DEFAULT 'team',
    avatar_url    text,
    totp_secret   text,
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz,

    CONSTRAINT users_role_check CHECK (role IN ('admin', 'team', 'client'))
);

-- Unicite de l'email sur les seuls comptes vivants : un compte supprime ne doit
-- pas bloquer la recreation, mais deux comptes actifs ne peuvent pas coexister.
CREATE UNIQUE INDEX users_email_unique ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX users_role_idx ON users (role) WHERE deleted_at IS NULL;

-- Les refresh tokens sont stockes haches : une fuite de la table ne permet pas
-- de rejouer une session.
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    user_agent text        NOT NULL DEFAULT '',
    ip         inet,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX refresh_tokens_hash_unique ON refresh_tokens (token_hash);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
