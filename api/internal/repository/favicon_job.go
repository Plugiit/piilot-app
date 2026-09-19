package repository

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/favicon"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// Reglages de la recuperation des logos d'apps.
const (
	// FaviconInterval : le job passe souvent au debut — une app qu'on vient
	// d'ajouter doit avoir son logo dans la minute, pas le lendemain — et ne
	// coute rien quand il n'y a rien a faire, la requete ne ramenant aucune
	// ligne grace a l'index partiel.
	FaviconInterval = time.Minute

	// FaviconRetry : delai avant de retenter une app dont la recuperation a
	// echoue. Un site injoignable le reste souvent un moment, et insister
	// chaque minute ne ferait que cogner a une porte fermee.
	FaviconRetry = 6 * time.Hour

	// FaviconBatch borne un passage. Chaque app coute un aller-reseau : en
	// traiter une poignee laisse le job court et previsible.
	FaviconBatch = 5
)

// StartFaviconFetch lance la recuperation periodique des logos en tache de fond
// et rend la main immediatement. La goroutine s'arrete avec le contexte.
//
// Un job et non une recuperation au moment de l'enregistrement : aller chercher
// une adresse tierce pendant qu'un ecran attend, c'est un ecran suspendu a un
// serveur qu'on ne maitrise pas. Ici, l'app est creee tout de suite et son logo
// arrive apres.
func StartFaviconFetch(
	ctx context.Context,
	pool *pgxpool.Pool,
	files storage.Store,
	log *slog.Logger,
) {
	fetcher := favicon.New()

	go func() {
		ticker := time.NewTicker(FaviconInterval)
		defer ticker.Stop()

		// Un passage tout de suite, avant le premier tick : au redemarrage, les
		// apps en attente ne doivent pas patienter une minute de plus.
		fetchPendingFavicons(ctx, pool, files, fetcher, log)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fetchPendingFavicons(ctx, pool, files, fetcher, log)
			}
		}
	}()
}

// fetchPendingFavicons execute un passage.
func fetchPendingFavicons(
	ctx context.Context,
	pool *pgxpool.Pool,
	files storage.Store,
	fetcher *favicon.Fetcher,
	log *slog.Logger,
) {
	// Delai de garde propre au passage : une base ou un site lent ne doit pas
	// retenir la goroutine jusqu'au tick suivant.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	q := db.New(pool)

	retryBefore := time.Now().Add(-FaviconRetry)

	rows, err := q.ListSidebarAppsNeedingFavicon(ctx, db.ListSidebarAppsNeedingFaviconParams{
		RetryBefore: &retryBefore,
		PageSize:    FaviconBatch,
	})
	if err != nil {
		log.Error("recuperation des logos : lecture impossible", "error", err)

		return
	}

	for _, row := range rows {
		icon, err := fetcher.Fetch(ctx, row.Url)
		if err != nil {
			// Un site sans favicon n'est pas un incident : on inscrit la
			// tentative pour ne pas y revenir tout de suite, et on passe.
			if markErr := q.MarkSidebarAppFaviconAttempted(ctx, row.ID); markErr != nil {
				log.Error("recuperation des logos : marquage impossible",
					"app", row.ID, "error", markErr)
			}

			// Une adresse refusee se dit plus fort : c'est soit une erreur de
			// saisie, soit quelqu'un qui cherche a faire sortir l'API vers le
			// reseau local.
			if errors.Is(err, favicon.ErrBlocked) {
				log.Warn("recuperation des logos : adresse refusee",
					"app", row.ID, "url", row.Url, "error", err)
			} else {
				log.Info("recuperation des logos : aucune icone",
					"app", row.ID, "url", row.Url, "error", err)
			}

			continue
		}

		key, _, err := files.Save(bytes.NewReader(icon.Content), int64(len(icon.Content)))
		if err != nil {
			log.Error("recuperation des logos : enregistrement impossible",
				"app", row.ID, "error", err)

			continue
		}

		err = q.SetSidebarAppFavicon(ctx, db.SetSidebarAppFaviconParams{
			ID:      row.ID,
			LogoKey: &key,
		})
		if err != nil {
			// Le fichier ne sera jamais designe : l'effacer evite de laisser un
			// orphelin sur le disque.
			_ = files.Remove(key)
			log.Error("recuperation des logos : rattachement impossible",
				"app", row.ID, "error", err)

			continue
		}

		log.Info("logo recupere", "app", row.ID, "url", row.Url, "type", icon.ContentType)
	}
}
