-- Chrono : le temps qui court sur une tache ou un projet.
--
-- Un seul par personne : on ne travaille pas sur deux choses a la fois, et
-- deux chronos qui tournent compteraient le meme temps deux fois. A l'arret,
-- le chrono devient une saisie de temps ordinaire et disparait. Il vit en
-- base, pas dans le navigateur : il suit la personne d'un onglet ou d'un poste
-- a l'autre, et survit a une fermeture.
CREATE TABLE time_timers (
    user_id    uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    task_id    uuid REFERENCES tasks (id) ON DELETE SET NULL,
    note       text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now()
);
