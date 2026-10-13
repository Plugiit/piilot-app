-- Le rapport mensuel de maintenance.
--
-- Un rapport porte ses donnees figees (payload) : tout ce qu'il montre est
-- copie a sa generation, rien n'est relu ensuite. Un rapport envoye ne
-- change plus jamais ; un rapport valide ne change pas sans repasser en
-- brouillon. Le declencheur le garantit, quoi que fasse le code.

CREATE TABLE report_settings (
    id         integer PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    -- Envoi automatique, desactive : un rapport part apres relecture. Active,
    -- les brouillons du mois sont valides et envoyes le jour dit.
    auto_send  boolean NOT NULL DEFAULT false,
    send_day   integer NOT NULL DEFAULT 5 CHECK (send_day BETWEEN 1 AND 28),
    -- Jours d'attente de la photo d'Umami avant de generer sans l'audience.
    wait_days  integer NOT NULL DEFAULT 3 CHECK (wait_days BETWEEN 0 AND 10),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO report_settings (id) VALUES (1);

CREATE TABLE reports (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id       uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    month            date NOT NULL CHECK (extract(day FROM month) = 1),
    status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'validated', 'sent')),
    -- La formule telle qu'elle etait ce mois-la.
    plan_name        text NOT NULL,
    included_minutes integer NOT NULL,
    sections         text[] NOT NULL,
    payload          jsonb NOT NULL,
    -- Le « mot du mois », redige par l'agence.
    editorial        text NOT NULL DEFAULT '',
    generated_at     timestamptz NOT NULL DEFAULT now(),
    validated_at     timestamptz,
    validated_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    sent_at          timestamptz,
    recipients       text[] NOT NULL DEFAULT '{}',
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, month)
);
CREATE INDEX reports_month_idx ON reports (month DESC, status);
CREATE INDEX reports_project_idx ON reports (project_id, month DESC);

CREATE FUNCTION reports_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'sent' THEN
        -- Seul l'effacement d'un valideur supprime (compte efface) passe.
        IF (to_jsonb(NEW) - 'validated_by') IS DISTINCT FROM (to_jsonb(OLD) - 'validated_by')
           OR (NEW.validated_by IS NOT NULL AND NEW.validated_by IS DISTINCT FROM OLD.validated_by) THEN
            RAISE EXCEPTION 'rapport envoye : il ne change plus' USING ERRCODE = 'check_violation';
        END IF;
    ELSIF OLD.status = 'validated' AND NEW.status = 'validated'
          AND (NEW.payload IS DISTINCT FROM OLD.payload OR NEW.editorial IS DISTINCT FROM OLD.editorial) THEN
        RAISE EXCEPTION 'rapport valide : repassez-le en brouillon pour le modifier' USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER reports_guard
    BEFORE UPDATE ON reports
    FOR EACH ROW EXECUTE FUNCTION reports_guard();

-- Les destinataires : les comptes du portail du client, moins ceux qu'on
-- exclut, plus des adresses ajoutees au projet.
CREATE TABLE project_report_recipients (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    email      citext NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, email)
);

CREATE TABLE project_report_opt_outs (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
