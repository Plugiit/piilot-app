package repository

import (
	"context"
	"log/slog"
	"time"
)

// uptimeTick : la tache regarde toutes les quinze secondes s'il faut
// relever. L'intervalle reel (1 a 5 minutes) se regle dans le module ; une
// demande « Tester » est servie au tick suivant.
const uptimeTick = 15 * time.Second

// UptimeWorker est ce que la tache attend du module de disponibilite.
type UptimeWorker interface {
	PollDue(ctx context.Context) bool
	Poll(ctx context.Context) error
}

// StartUptime releve le service de surveillance. C'est le seul endroit ou
// Piilot l'appelle : les ecrans lisent ce que la releve a ecrit.
func StartUptime(ctx context.Context, w UptimeWorker, log *slog.Logger) {
	go every(ctx, uptimeTick, func() {
		pollCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		if !w.PollDue(pollCtx) {
			return
		}
		if err := w.Poll(pollCtx); err != nil {
			log.Warn("releve de disponibilite", "error", err)
		}
	})
}
