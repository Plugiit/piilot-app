-- Maintenance : le journal des interventions et les formules.
--
-- Une intervention est une saisie de temps qualifiee, pas une table a part :
-- le temps n'est saisi qu'une fois, il compte dans les rapports de temps
-- comme dans la maintenance. Seules les saisies marquees comptent sur le
-- temps inclus de la formule.

ALTER TABLE time_entries
    ADD COLUMN intervention_kind text
        CHECK (intervention_kind IN ('contenu', 'correctif', 'mise_a_jour', 'support', 'autre')),
    -- Le client la voit dans son espace et dans son rapport, avec sa note.
    ADD COLUMN client_visible boolean NOT NULL DEFAULT false,
    -- Ce que le client lit : la note interne reste a l'equipe.
    ADD COLUMN client_note text NOT NULL DEFAULT '',
    ADD CONSTRAINT time_entries_visible_is_intervention
        CHECK (NOT client_visible OR intervention_kind IS NOT NULL);

CREATE INDEX time_entries_interventions_idx
    ON time_entries (project_id, spent_on DESC)
    WHERE intervention_kind IS NOT NULL AND deleted_at IS NULL;

-- Les formules : un nom, un temps inclus par mois, non cumulable, et les
-- sections du rapport mensuel. Les sections de la 1.6 (mises a jour,
-- sauvegardes, domaine) sont admises des maintenant : une formule les porte,
-- le rapport les montre quand leurs donnees existent.
CREATE TABLE maintenance_plans (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             text NOT NULL,
    included_minutes integer NOT NULL CHECK (included_minutes BETWEEN 0 AND 100000),
    sections         text[] NOT NULL DEFAULT ARRAY['availability', 'audience', 'interventions', 'deployments'],
    active           boolean NOT NULL DEFAULT true,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (sections <@ ARRAY['availability', 'audience', 'interventions', 'deployments', 'updates', 'backups', 'domain'])
);
CREATE UNIQUE INDEX maintenance_plans_name_idx ON maintenance_plans (lower(name));

-- La formule d'un projet, par periodes : changer de formule ferme la
-- periode en cours et en ouvre une autre, les mois passes gardent la leur.
-- Une seule periode ouverte par projet.
CREATE TABLE project_maintenance (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    plan_id    uuid NOT NULL REFERENCES maintenance_plans (id) ON DELETE RESTRICT,
    starts_on  date NOT NULL,
    ends_on    date,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_on IS NULL OR ends_on >= starts_on)
);
CREATE UNIQUE INDEX project_maintenance_open_idx ON project_maintenance (project_id) WHERE ends_on IS NULL;
CREATE INDEX project_maintenance_project_idx ON project_maintenance (project_id, starts_on DESC);

-- Le temps consomme par projet et par mois, en minutes d'intervention.
-- Tenu par declencheur : la liste et le rapport le lisent, rien ne le
-- recalcule a l'affichage.
CREATE TABLE maintenance_months (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    month      date NOT NULL,
    minutes    integer NOT NULL DEFAULT 0,
    PRIMARY KEY (project_id, month)
);

CREATE FUNCTION refresh_maintenance_month(target uuid, day date) RETURNS void LANGUAGE sql AS $$
    INSERT INTO maintenance_months (project_id, month, minutes)
    SELECT target, date_trunc('month', day)::date, coalesce(sum(minutes), 0)::integer
    FROM time_entries
    WHERE project_id = target AND deleted_at IS NULL AND intervention_kind IS NOT NULL
      AND spent_on >= date_trunc('month', day)::date
      AND spent_on < (date_trunc('month', day) + interval '1 month')::date
    ON CONFLICT (project_id, month) DO UPDATE SET minutes = EXCLUDED.minutes;
$$;

-- Une saisie qui entre, sort, change de mois, de projet ou de nature remet
-- a jour les mois qu'elle touche — l'ancien comme le nouveau.
CREATE FUNCTION time_entries_maintenance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP <> 'INSERT' AND OLD.intervention_kind IS NOT NULL THEN
        PERFORM refresh_maintenance_month(OLD.project_id, OLD.spent_on);
    END IF;
    IF TG_OP <> 'DELETE' AND NEW.intervention_kind IS NOT NULL THEN
        PERFORM refresh_maintenance_month(NEW.project_id, NEW.spent_on);
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER time_entries_maintenance
    AFTER INSERT OR DELETE OR UPDATE OF project_id, spent_on, minutes, intervention_kind, deleted_at ON time_entries
    FOR EACH ROW EXECUTE FUNCTION time_entries_maintenance();

-- Les formules et les rapports aux admins.
INSERT INTO permissions (code, label) VALUES
    ('reports.write', 'Gérer les formules de maintenance et les rapports mensuels');
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.code = 'admin' AND p.code = 'reports.write';
