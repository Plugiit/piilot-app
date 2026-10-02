-- name: ListProjectWorkload :many
-- Graphique « Charge par projet » : les projets en cours les plus avances
-- dans leur budget.
--
-- Tri sur la part consommee, pas sur les heures : c'est la derive que le
-- graphique doit montrer d'abord. Huit barres tiennent dans la demi-largeur
-- du tableau de bord ; au-dela, la liste des projets triee par budget prend
-- le relais. Lu sur les heures precalculees, aucune somme au rendu.
SELECT
    p.id,
    p.name,
    c.name AS client_name,
    p.hours_sold,
    p.hours_spent
FROM projects p
JOIN clients c ON c.id = p.client_id
WHERE p.deleted_at IS NULL
  AND NOT p.is_internal
  AND p.status <> 'livre'
  AND p.hours_sold > 0
ORDER BY p.hours_spent / p.hours_sold DESC, p.id
LIMIT 8;

-- name: ListTeamDay :many
-- Carte « Equipe aujourd'hui » : ce que chaque membre a saisi aujourd'hui et
-- depuis lundi, et sur quoi il a pointe en dernier.
--
-- Une somme au rendu, contrairement a la regle du projet. Elle est admise
-- parce qu'elle ne porte que sur la semaine en cours — quelques dizaines de
-- saisies par personne — et passe par l'index (user_id, spent_on). Un
-- precalcul par personne et par jour dupliquerait la table des saisies sans
-- rien faire gagner a cette echelle.
SELECT
    u.id,
    u.firstname,
    u.lastname,
    u.avatar_url,
    coalesce(sum(e.minutes) FILTER (WHERE e.spent_on = sqlc.arg('today')::date), 0)::bigint AS today_minutes,
    coalesce(sum(e.minutes) FILTER (WHERE e.spent_on = sqlc.arg('today')::date AND NOT p.is_internal), 0)::bigint
        AS today_billable_minutes,
    coalesce(sum(e.minutes), 0)::bigint AS week_minutes,
    -- Chaine vide sans saisie du jour : la jointure laterale est externe, et
    -- sqlc ne sait pas qu'elle peut rendre NULL.
    coalesce(last.project_name, '')::text AS last_project,
    coalesce(last.task_title, '')::text   AS last_task
FROM users u
LEFT JOIN time_entries e
       ON e.user_id = u.id
      AND e.deleted_at IS NULL
      AND e.spent_on BETWEEN sqlc.arg('week_start')::date AND sqlc.arg('today')::date
LEFT JOIN projects p ON p.id = e.project_id
-- Derniere saisie du jour : ce qui dit « sur quoi il travaille ».
LEFT JOIN LATERAL (
    SELECT lp.name AS project_name, lt.title AS task_title
    FROM time_entries le
    JOIN projects lp ON lp.id = le.project_id
    LEFT JOIN tasks lt ON lt.id = le.task_id
    WHERE le.user_id = u.id
      AND le.deleted_at IS NULL
      AND le.spent_on = sqlc.arg('today')::date
    ORDER BY le.created_at DESC
    LIMIT 1
) AS last ON true
WHERE u.deleted_at IS NULL
  AND u.role IN ('admin', 'team')
GROUP BY u.id, last.project_name, last.task_title
ORDER BY today_minutes DESC, week_minutes DESC, u.firstname, u.lastname, u.id
LIMIT 50;

-- name: ListDailyActivity :many
-- Carte « Activite par jour » : une ligne par jour actif de la periode,
-- precalculee par declencheur. Les jours sans ligne sont des jours vides.
SELECT day, tasks_done, tickets_opened, deliverables_submitted
FROM daily_activity
WHERE day BETWEEN sqlc.arg('from_day')::date AND sqlc.arg('to_day')::date
ORDER BY day
LIMIT 366;
