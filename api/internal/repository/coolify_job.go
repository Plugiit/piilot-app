package repository

import (
	"context"
	"log/slog"
	"time"
)

// coolifyTick : la tache regarde toutes les quinze secondes s'il faut
// relever. L'intervalle reel (15 minutes a 24 heures) se regle dans le
// module ; une demande « Tester » est servie au tick suivant.
const coolifyTick = 15 * time.Second

// CoolifyWorker est ce que la tache attend du module de deploiements.
type CoolifyWorker interface {
	PollDue(ctx context.Context) bool
	Poll(ctx context.Context) error
}

// StartCoolify releve les applications de Coolify. C'est le seul endroit ou
// Piilot l'appelle : les ecrans lisent ce que la releve a ecrit.
func StartCoolify(ctx context.Context, w CoolifyWorker, log *slog.Logger) {
	go every(ctx, coolifyTick, func() {
		pollCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if !w.PollDue(pollCtx) {
			return
		}
		if err := w.Poll(pollCtx); err != nil {
			log.Warn("releve des deploiements", "error", err)
		}
	})
}
