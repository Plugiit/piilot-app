-- RBAC : les roles et leurs permissions vivent en base, pas en dur dans le
-- code. Un role peut gagner ou perdre une permission sans redeploiement, et
-- l'API reste la seule autorite sur ce qu'un compte peut faire.
--
-- Un utilisateur porte UN role (colonne users.role, deja presente). Le lien
-- many-to-many est entre roles et permissions, pas entre users et roles : un
-- compte qui cumule deux metiers est un cas qui n'existe pas dans une agence
-- de cette taille, et le supposer couterait une jointure de plus sur chaque
-- requete authentifiee.

CREATE TABLE roles (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code       text        NOT NULL,
    label      text        NOT NULL,
    -- Un role systeme est reference par le code (aiguillage du front, gardes
    -- d'espace) : le supprimer casserait l'application.
    is_system  boolean     NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX roles_code_unique ON roles (code);

CREATE TABLE permissions (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Code stable en <domaine>.<action> : c'est lui que le code interroge,
    -- jamais le libelle ni l'identifiant.
    code       text        NOT NULL,
    label      text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX permissions_code_unique ON permissions (code);

CREATE TABLE role_permissions (
    role_id       uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    created_at    timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (role_id, permission_id)
);

-- Sens de lecture reel : « quelles permissions pour ce role ». La cle primaire
-- couvre deja (role_id, permission_id) ; cet index sert le sens inverse, pour
-- repondre a « qui detient cette permission » sans balayage.
CREATE INDEX role_permissions_permission_idx ON role_permissions (permission_id);

-- Roles systeme. Les codes correspondent a ceux que porte deja users.role et
-- sur lesquels le front aiguille (isInternal / homeFor).
INSERT INTO roles (code, label, is_system) VALUES
    ('admin',  'Administrateur', true),
    ('team',   'Equipe',         true),
    ('client', 'Client',         true);

-- Vocabulaire de permissions du perimetre acte : PM, CRM et portail client.
-- Les endpoints correspondants n'existent pas encore ; ce sont les codes que
-- les modules a venir consommeront.
INSERT INTO permissions (code, label) VALUES
    ('users.read',            'Consulter les comptes'),
    ('users.write',           'Creer et modifier les comptes'),
    ('roles.read',            'Consulter les roles et permissions'),
    ('roles.write',           'Modifier les roles et permissions'),
    ('projects.read',         'Consulter les projets'),
    ('projects.write',        'Creer et modifier les projets'),
    ('tasks.read',            'Consulter les taches'),
    ('tasks.write',           'Creer et modifier les taches'),
    ('time.read',             'Consulter le temps passe'),
    ('time.write',            'Saisir du temps passe'),
    ('deliverables.read',     'Consulter les livrables'),
    ('deliverables.write',    'Creer et modifier les livrables'),
    ('deliverables.validate', 'Valider ou refuser un livrable'),
    ('tickets.read',          'Consulter les tickets'),
    ('tickets.write',         'Creer et commenter les tickets'),
    ('clients.read',          'Consulter les clients'),
    ('clients.write',         'Creer et modifier les clients');

-- admin : tout, sans exception.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin';

-- team : le quotidien de l'agence, sans l'administration des comptes ni des
-- droits — c'est la separation qui empeche un compte courant de s'octroyer
-- des permissions.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'team'
  AND p.code NOT IN ('users.write', 'roles.write');

-- client : le portail seul. Lecture de ses projets, validation de ses
-- livrables, tickets. Aucune permission d'ecriture sur le back-office.
--
-- Ces permissions disent ce qu'un client peut faire, jamais SUR QUOI : le
-- filtrage par client de l'appelant reste a la charge de chaque endpoint du
-- portail, systematiquement.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'client'
  AND p.code IN (
      'projects.read',
      'deliverables.read',
      'deliverables.validate',
      'tickets.read',
      'tickets.write'
  );

-- users.role devient une reference vers roles.code : la liste des roles n'est
-- plus figee dans le schema. Les lignes sont inserees avant l'ajout de la
-- contrainte, sans quoi les comptes existants la violeraient.
ALTER TABLE users DROP CONSTRAINT users_role_check;

ALTER TABLE users
    ADD CONSTRAINT users_role_fkey
    FOREIGN KEY (role) REFERENCES roles (code) ON UPDATE CASCADE;
