package repository

import (
	"context"
	"log/slog"
	"time"
)

// siteUpdatesTick : un lancement demande depuis l'ecran part dans la minute.
const siteUpdatesTick = time.Minute

// SiteUpdatesWorker est ce que la tache attend des mises a jour des sites.
type SiteUpdatesWorker interface {
	Tick(ctx context.Context) error
}

// StartSiteUpdates lance les mises a jour demandees et surveille leurs comptes
// rendus.
func StartSiteUpdates(ctx context.Context, w SiteUpdatesWorker, log *slog.Logger) {
	go every(ctx, siteUpdatesTick, func() {
		tickCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		if err := w.Tick(tickCtx); err != nil {
			log.Warn("mises a jour des sites", "error", err)
		}
	})
}
