package repository

import (
	"context"
	"log/slog"
	"time"
)

// reportTick : les brouillons du mois ecoule se creent des le 1er, et
// l'envoi automatique, s'il est active, part le jour dit. Un quart d'heure
// suffit a ce rythme-la.
const reportTick = 15 * time.Minute

// ReportWorker est ce que la tache attend du rapport mensuel.
type ReportWorker interface {
	Tick(ctx context.Context) error
}

// StartReports genere les rapports mensuels et, si c'est regle, les envoie.
func StartReports(ctx context.Context, w ReportWorker, log *slog.Logger) {
	go every(ctx, reportTick, func() {
		tickCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := w.Tick(tickCtx); err != nil {
			log.Warn("rapports mensuels", "error", err)
		}
	})
}
