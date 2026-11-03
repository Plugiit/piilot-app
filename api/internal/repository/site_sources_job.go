package repository

import (
	"context"
	"log/slog"
	"time"
)

// siteSourcesTick : une sauvegarde manquante se dit dans le quart d'heure. Le
// releve RDAP, lui, n'a lieu qu'une fois par semaine par domaine.
const siteSourcesTick = 15 * time.Minute

// SiteSourcesWorker est ce que la tache attend des sauvegardes et domaines.
type SiteSourcesWorker interface {
	Tick(ctx context.Context) error
}

// StartSiteSources surveille les sauvegardes des sites et releve leurs domaines.
func StartSiteSources(ctx context.Context, w SiteSourcesWorker, log *slog.Logger) {
	go every(ctx, siteSourcesTick, func() {
		tickCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := w.Tick(tickCtx); err != nil {
			log.Warn("sauvegardes et domaines des sites", "error", err)
		}
	})
}
