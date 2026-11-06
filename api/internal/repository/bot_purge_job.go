package repository

import (
	"context"
	"log/slog"
	"time"
)

// botPurgeTick : une fois par heure. Un code expire ne vaut deja plus rien ;
// l'effacer vite borne seulement ce qu'une fuite de la table montrerait.
const botPurgeTick = time.Hour

// Purger efface ce qui ne doit plus etre garde.
type Purger interface {
	Purge(ctx context.Context) error
}

// StartBotPurge efface les codes de liaison expires et les vieux evenements du
// flux du bot. Plusieurs instances peuvent la faire tourner ensemble : chaque
// passage est un DELETE, rejouable sans effet.
func StartBotPurge(ctx context.Context, log *slog.Logger, purgers ...Purger) {
	go every(ctx, botPurgeTick, func() {
		runCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		for _, p := range purgers {
			if err := p.Purge(runCtx); err != nil {
				log.Warn("purge des donnees du bot", "error", err)
			}
		}
	})
}
