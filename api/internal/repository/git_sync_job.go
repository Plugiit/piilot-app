package repository

import (
	"context"
	"log/slog"
	"time"
)

// gitSyncTick : la tache regarde chaque minute ce qui est du ou demande. La
// synchronisation d'un projet ne se fait qu'une fois par jour ; le tick sert
// surtout a servir vite un « Synchroniser maintenant ».
const gitSyncTick = time.Minute

// GitSyncWorker est ce que la tache attend du rattrapage Git.
type GitSyncWorker interface {
	Tick(ctx context.Context) error
}

// StartGitSync rattrape les pull requests que le webhook a manquees. C'est le
// seul endroit ou Piilot appelle GitHub pour cela.
func StartGitSync(ctx context.Context, w GitSyncWorker, log *slog.Logger) {
	go every(ctx, gitSyncTick, func() {
		tickCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := w.Tick(tickCtx); err != nil {
			log.Warn("rattrapage Git", "error", err)
		}
	})
}
