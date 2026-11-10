-- Le connecteur Claude : Piilot sert d'autorisation OAuth 2.1 a Claude
-- (claude.ai, Desktop, Claude Code), qui agit ensuite au nom d'un compte par le
-- serveur MCP de /mcp. Les droits restent ceux du compte, relus a chaque appel.

INSERT INTO permissions (code, label) VALUES
    ('claude.connect', 'Connecter Claude à son compte');

-- Equipe et administration. Pas le portail : les outils de Claude sont ceux
-- du back-office.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code IN ('admin', 'team') AND p.code = 'claude.connect';

-- Demandes d'autorisation validees, en attente du consentement : quelques
-- minutes, le temps de se connecter et de cliquer. browser_hash : empreinte du
-- jeton pose en cookie dans le navigateur qui a lance la demande. Seul ce
-- navigateur peut la trancher : un lien de consentement transmis a quelqu'un
-- d'autre ne vaut rien.
CREATE TABLE oauth_requests (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id      text NOT NULL,
    redirect_uri   text NOT NULL,
    state          text,
    code_challenge text NOT NULL,
    scopes         text[] NOT NULL,
    resource       text NOT NULL,
    browser_hash   bytea NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    expires_at     timestamptz NOT NULL
);
CREATE INDEX oauth_requests_expires_idx ON oauth_requests (expires_at);

-- Une autorisation : un compte, un client, des portees. Chaque consentement en
-- cree une ; la revoquer coupe tous ses jetons.
CREATE TABLE oauth_grants (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_id    text NOT NULL,
    scopes       text[] NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);
CREATE INDEX oauth_grants_user_idx ON oauth_grants (user_id) WHERE revoked_at IS NULL;

-- Codes d'autorisation : empreinte seule, usage unique, deux minutes. Lie au
-- retour, au defi PKCE et a la ressource demandes.
CREATE TABLE oauth_codes (
    id             uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code_hash      bytea NOT NULL,
    grant_id       uuid NOT NULL REFERENCES oauth_grants (id) ON DELETE CASCADE,
    redirect_uri   text NOT NULL,
    code_challenge text NOT NULL,
    resource       text NOT NULL,
    expires_at     timestamptz NOT NULL,
    used_at        timestamptz
);
CREATE UNIQUE INDEX oauth_codes_hash_unique ON oauth_codes (code_hash);
CREATE INDEX oauth_codes_expires_idx ON oauth_codes (expires_at);

-- Jetons d'acces et de renouvellement : empreinte seule. Un jeton de
-- renouvellement deja servi garde sa ligne jusqu'a expiration : le revoir
-- trahit un vol, et coupe l'autorisation.
CREATE TABLE oauth_tokens (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash bytea NOT NULL,
    grant_id   uuid NOT NULL REFERENCES oauth_grants (id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('access', 'refresh')),
    resource   text NOT NULL,
    scopes     text[] NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);
CREATE UNIQUE INDEX oauth_tokens_hash_unique ON oauth_tokens (token_hash);
CREATE INDEX oauth_tokens_grant_idx ON oauth_tokens (grant_id);
CREATE INDEX oauth_tokens_expires_idx ON oauth_tokens (expires_at);
