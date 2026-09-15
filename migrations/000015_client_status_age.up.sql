-- Anciennete dans l'etape du pipeline.
--
-- Un kanban commercial sert d'abord a reperer ce qui stagne : sans cette date,
-- rien ne distingue une carte arrivee hier d'une carte bloquee en « devis »
-- depuis six semaines, et les deux se lisent pareil.
--
-- La valeur par defaut date les clients existants du jour de cette migration.
-- Ce n'est pas un pis-aller : c'est 000014 qui leur a pose leur statut actuel,
-- ils sont donc bien dans cette etape depuis ce moment-la. Aucune histoire
-- n'est inventee.
--
-- NOT NULL plutot que nullable : une etape a toujours une date d'entree, et
-- l'admettre ici evite un *time.Time cote Go et un cas vide a l'ecran.
ALTER TABLE clients
    ADD COLUMN status_changed_at timestamptz NOT NULL DEFAULT now();

-- Le declencheur plutot que les requetes : `status` s'ecrit a DEUX endroits —
-- `UpdateClient`, qui renvoie tous les champs a chaque enregistrement, et
-- `UpdateClientStatus`, que le glisser du kanban appelle. Poser la date dans
-- les requetes obligerait a y penser aux deux, et surtout `UpdateClient`
-- rajeunirait la carte a chaque correction de telephone, puisqu'il reecrit le
-- statut meme inchange.
--
-- `IS DISTINCT FROM` et non `<>` : les deux valeurs sont NOT NULL aujourd'hui,
-- mais l'operateur reste juste si cela change, la ou `<>` rendrait NULL et
-- laisserait la date en place sans bruit.
CREATE OR REPLACE FUNCTION touch_client_status_changed()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status IS DISTINCT FROM OLD.status THEN
        NEW.status_changed_at = now();
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER clients_status_changed
    BEFORE UPDATE ON clients
    FOR EACH ROW
    EXECUTE FUNCTION touch_client_status_changed();
