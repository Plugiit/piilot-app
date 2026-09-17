-- Applications jointes depuis le rail.

-- name: ListSidebarApps :many
-- Toutes les apps, dans l'ordre du rail.
--
-- Bornee en dur : c'est une liste de navigation, pas un ecran. Vingt raccourcis
-- tiennent dans un rail, au-dela ce n'est plus un rail. La regle du projet veut
-- un LIMIT partout — ici il n'a pas de page suivante, il a une fin.
SELECT id, name, url, logo_key, logo_is_favicon, color, position, created_at, updated_at
FROM sidebar_apps
WHERE deleted_at IS NULL
ORDER BY position, name, id
LIMIT 50;

-- name: GetSidebarApp :one
SELECT id, name, url, logo_key, logo_is_favicon, color, position, created_at, updated_at
FROM sidebar_apps
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: CreateSidebarApp :one
-- Le rang par defaut place la nouvelle app en queue : on ajoute au bout, on
-- reordonne ensuite si besoin.
INSERT INTO sidebar_apps (name, url, color, position)
VALUES (
    sqlc.arg('name'), sqlc.arg('url'), sqlc.arg('color'),
    coalesce((SELECT max(position) + 1 FROM sidebar_apps WHERE deleted_at IS NULL), 1)
)
RETURNING id, name, url, logo_key, logo_is_favicon, color, position, created_at, updated_at;

-- name: UpdateSidebarApp :one
UPDATE sidebar_apps
SET name       = sqlc.arg('name'),
    url        = sqlc.arg('url'),
    color      = sqlc.arg('color'),
    position   = sqlc.arg('position'),
    -- Changer d'adresse rouvre la recuperation : le favicon du nouveau site
    -- n'a aucune raison d'etre celui de l'ancien.
    favicon_attempted_at = CASE WHEN url IS DISTINCT FROM sqlc.arg('url')
                                THEN NULL ELSE favicon_attempted_at END,
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, url, logo_key, logo_is_favicon, color, position, created_at, updated_at;

-- name: SetSidebarAppLogo :one
-- L'appelant lit l'app avant d'appeler : c'est lui qui connait l'ancienne cle
-- et qui efface le fichier qu'elle designait, sans quoi le magasin garderait
-- tous les logos jamais deposes.
UPDATE sidebar_apps
SET logo_key        = sqlc.narg('logo_key'),
    logo_is_favicon = sqlc.arg('is_favicon'),
    updated_at      = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, url, logo_key, logo_is_favicon, color, position, created_at, updated_at;

-- name: DeleteSidebarApp :one
-- Suppression douce, comme partout. Rend la cle du logo pour que l'appelant
-- decide du sort du fichier.
UPDATE sidebar_apps
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING logo_key;

-- name: SidebarAppLogoKeyInUse :one
-- Vrai quand la cle designe encore le logo d'une app vivante : c'est ce qui
-- autorise a servir le fichier.
SELECT EXISTS (
    SELECT 1 FROM sidebar_apps
    WHERE logo_key = sqlc.arg('logo_key') AND deleted_at IS NULL
);

-- name: ListSidebarAppsNeedingFavicon :many
-- Apps dont le logo reste a chercher.
--
-- Celles qui n'ont pas de logo et qu'on n'a jamais tentees, ou tentees avant la
-- borne passee en parametre. Un echec n'est donc pas rejoue en boucle : il
-- attend son tour de reessai.
--
-- Bornee : un passage traite une poignee d'apps, le suivant prendra la suite.
SELECT id, url
FROM sidebar_apps
WHERE deleted_at IS NULL
  AND logo_key IS NULL
  AND (favicon_attempted_at IS NULL OR favicon_attempted_at < sqlc.arg('retry_before'))
ORDER BY favicon_attempted_at NULLS FIRST, position
LIMIT sqlc.arg('page_size');

-- name: MarkSidebarAppFaviconAttempted :exec
-- Inscrit la tentative, qu'elle ait abouti ou non.
UPDATE sidebar_apps
SET favicon_attempted_at = now()
WHERE id = sqlc.arg('id');

-- name: SetSidebarAppFavicon :exec
-- Range le favicon recupere.
--
-- La clause sur `logo_key` protege un depot manuel : si quelqu'un a pose un
-- logo entre la lecture et l'ecriture, la recuperation ne l'ecrase pas.
UPDATE sidebar_apps
SET logo_key             = sqlc.arg('logo_key'),
    logo_is_favicon      = true,
    favicon_attempted_at = now(),
    updated_at           = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL AND logo_key IS NULL;

-- name: ResetSidebarAppFavicon :one
-- Repasse le logo en automatique.
--
-- Efface le logo courant et rouvre la recuperation : le job exige un logo nul,
-- et ne retente qu'au bout de plusieurs heures. Remettre la date a zero le fait
-- reprendre l'app a son prochain passage, donc dans la minute.
--
-- La recuperation elle-meme n'a pas sa place ici : elle sort vers un serveur
-- tiers, et aucune requete HTTP de l'application ne doit attendre cela.
UPDATE sidebar_apps
SET logo_key             = NULL,
    logo_is_favicon      = false,
    favicon_attempted_at = NULL,
    updated_at           = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, name, url, logo_key, logo_is_favicon, color, position, created_at, updated_at;
