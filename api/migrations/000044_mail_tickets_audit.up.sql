-- 1.2 : les tickets par e-mail, les reponses types, le journal d'audit.

-- Reglages de l'e-mail entrant : une seule ligne. Les variables INBOUND_*
-- de l'environnement priment sur ce qui est ecrit ici ; l'ecran les montre
-- alors en lecture seule.
CREATE TABLE inbound_mail_settings (
    id                integer PRIMARY KEY DEFAULT 1,
    -- Adresse de support : support@agence.fr. Les reponses arrivent sur
    -- support+t47.<signature>@agence.fr, que la boite doit accepter.
    address           text NOT NULL DEFAULT '',
    imap_enabled      boolean NOT NULL DEFAULT false,
    imap_host         text NOT NULL DEFAULT '',
    imap_port         integer NOT NULL DEFAULT 993,
    -- tls (993) ou none (serveur local de test).
    imap_security     text NOT NULL DEFAULT 'tls',
    imap_username     text NOT NULL DEFAULT '',
    -- Chiffre (AES-GCM, cle derivee de JWT_SECRET) : jamais en clair en base.
    imap_password_enc bytea,
    imap_folder       text NOT NULL DEFAULT 'INBOX',
    -- Secret des webhooks des fournisseurs, tire au premier affichage.
    webhook_secret    text NOT NULL DEFAULT '',
    -- Ce que la releve a fait la derniere fois, pour l'ecran.
    last_poll_at      timestamptz,
    last_poll_error   text NOT NULL DEFAULT '',
    -- Un admin a demande une releve immediate.
    poll_requested_at timestamptz,
    updated_at        timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT inbound_mail_settings_singleton CHECK (id = 1),
    CONSTRAINT inbound_mail_settings_security_check CHECK (imap_security IN ('tls', 'none'))
);
INSERT INTO inbound_mail_settings (id) VALUES (1);

-- Les e-mails recus, quelle que soit leur porte : releve IMAP ou webhook.
-- Tout entre ici tel quel, puis une tache de fond le traite : aucune requete
-- HTTP n'attend qu'un e-mail soit range.
CREATE TABLE inbound_emails (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    received_at  timestamptz NOT NULL DEFAULT now(),
    -- imap, postmark, mailgun, brevo ou raw.
    source       text NOT NULL,
    -- Message-ID de l'e-mail, sans chevrons. Sert a ne jamais traiter deux
    -- fois le meme : une releve rejouee, un webhook renvoye.
    message_id   text NOT NULL DEFAULT '',
    from_address citext NOT NULL DEFAULT '',
    from_name    text NOT NULL DEFAULT '',
    subject      text NOT NULL DEFAULT '',
    -- Debut du texte et nombre de pieces jointes, lus a l'arrivee : la liste
    -- a trier les affiche sans relire chaque message.
    excerpt          text NOT NULL DEFAULT '',
    attachment_count integer NOT NULL DEFAULT 0,
    -- Le message complet, au format MIME.
    raw          bytea NOT NULL,
    -- pending : a traiter ; processed : range dans un ticket ; held : a trier
    -- a la main ; ignored : reponse automatique, rebond ; failed : illisible.
    status       text NOT NULL DEFAULT 'pending',
    -- Pourquoi il attend ou a ete ecarte : unknown_sender, project_to_choose,
    -- sender_mismatch, from_team, rate_limited, auto_reply…
    reason       text NOT NULL DEFAULT '',
    -- Client devine d'apres l'expediteur, pour proposer ses projets au tri.
    client_id    uuid REFERENCES clients (id) ON DELETE SET NULL,
    ticket_id    uuid REFERENCES tickets (id) ON DELETE SET NULL,
    attempts     integer NOT NULL DEFAULT 0,
    error        text NOT NULL DEFAULT '',
    processed_at timestamptz,

    CONSTRAINT inbound_emails_status_check
        CHECK (status IN ('pending', 'processed', 'held', 'ignored', 'failed')),
    CONSTRAINT inbound_emails_source_check
        CHECK (source IN ('imap', 'postmark', 'mailgun', 'brevo', 'raw'))
);
CREATE UNIQUE INDEX inbound_emails_message_id_idx ON inbound_emails (message_id) WHERE message_id <> '';
CREATE INDEX inbound_emails_pending_idx ON inbound_emails (received_at) WHERE status = 'pending';
CREATE INDEX inbound_emails_held_idx ON inbound_emails (received_at DESC) WHERE status = 'held';
CREATE INDEX inbound_emails_sender_idx ON inbound_emails (from_address, received_at DESC);

-- Un ticket ouvert par e-mail par quelqu'un qui n'a pas de compte sur le
-- portail : c'est a cette adresse que partent les reponses.
ALTER TABLE tickets
    ADD COLUMN requester_email citext,
    ADD COLUMN requester_name  text NOT NULL DEFAULT '';

-- Un message arrive par e-mail : il garde le nom et l'adresse de qui l'a
-- ecrit, meme sans compte.
ALTER TABLE ticket_messages
    ADD COLUMN via_email    boolean NOT NULL DEFAULT false,
    ADD COLUMN sender_name  text NOT NULL DEFAULT '',
    ADD COLUMN sender_email text NOT NULL DEFAULT '';

-- E-mails sortants filés : la reponse du client revient sur le bon ticket.
ALTER TABLE email_outbox
    ADD COLUMN reply_to    text NOT NULL DEFAULT '',
    ADD COLUMN message_id  text NOT NULL DEFAULT '',
    ADD COLUMN in_reply_to text NOT NULL DEFAULT '';

-- Reponses types : les demandes qui reviennent.
CREATE TABLE ticket_reply_templates (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    title      text NOT NULL,
    body       text NOT NULL,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT ticket_reply_templates_title_check CHECK (btrim(title) <> ''),
    CONSTRAINT ticket_reply_templates_body_check CHECK (btrim(body) <> '')
);

-- Journal d'audit : qui a fait quoi, quand, d'ou. En ajout seul.
CREATE TABLE audit_log (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    at          timestamptz NOT NULL DEFAULT now(),
    -- Le compte, et son adresse figee au moment du geste : elle se lit encore
    -- quand le compte a change d'adresse ou disparu.
    actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_email text NOT NULL DEFAULT '',
    action      text NOT NULL,
    target_type text NOT NULL DEFAULT '',
    target_id   text NOT NULL DEFAULT '',
    ip          text NOT NULL DEFAULT '',
    user_agent  text NOT NULL DEFAULT '',
    details     jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX audit_log_at_idx ON audit_log (at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log (actor_id, at DESC);
CREATE INDEX audit_log_action_idx ON audit_log (action, at DESC);

-- Une ligne d'audit ne se modifie jamais, et ne s'efface que par la purge de
-- retention, qui le declare dans sa transaction.
--
-- Une seule modification passe : la cle etrangere qui remet actor_id a nul
-- quand un compte est supprime. L'adresse figee dans actor_email reste, et
-- rien d'autre ne bouge.
CREATE FUNCTION audit_log_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('piilot.audit_purge', true) = 'on' THEN
        RETURN OLD;
    END IF;
    IF TG_OP = 'UPDATE' AND NEW.actor_id IS NULL
       AND ROW(NEW.id, NEW.at, NEW.actor_email, NEW.action, NEW.target_type, NEW.target_id, NEW.ip, NEW.user_agent, NEW.details)
           IS NOT DISTINCT FROM
           ROW(OLD.id, OLD.at, OLD.actor_email, OLD.action, OLD.target_type, OLD.target_id, OLD.ip, OLD.user_agent, OLD.details) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'audit_log est en ajout seul';
END;
$$;
CREATE TRIGGER audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_guard();

INSERT INTO permissions (code, label) VALUES
    ('audit.read', 'Consulter le journal d''audit');
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code = 'audit.read';

-- Un e-mail mis de cote previent ceux qui trient.
ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_kind_check CHECK (kind IN (
        'task_created',
        'task_status_changed',
        'task_assigned',
        'task_unassigned',
        'task_commented',
        'task_due_changed',
        'project_created',
        'ticket_created',
        'ticket_assigned',
        'ticket_replied',
        'ticket_status_changed',
        'deliverable_validated',
        'deliverable_feedback',
        'update_available',
        'backup_stale',
        'inbound_email_held'
    ));
