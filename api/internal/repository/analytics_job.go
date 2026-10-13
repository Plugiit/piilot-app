package repository

import (
	"context"
	"log/slog"
	"time"
)

// analyticsTick : la tache des statistiques passe toutes les quinze
// secondes. Chaque passage borne ce qu'il demande au service ; les rythmes
// reels — duree de vie du cache, visiteurs du moment — sont dans le service.
const analyticsTick = 15 * time.Second

// AnalyticsWorker est ce que la tache attend du module de statistiques.
type AnalyticsWorker interface {
	Tick(ctx context.Context) error
}

// StartAnalytics fait tourner le module de statistiques. C'est le seul
// endroit ou Piilot appelle le service de mesure d'audience.
func StartAnalytics(ctx context.Context, w AnalyticsWorker, log *slog.Logger) {
	go every(ctx, analyticsTick, func() {
		tickCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := w.Tick(tickCtx); err != nil {
			log.Warn("statistiques", "error", err)
		}
	})
}
