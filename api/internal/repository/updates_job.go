package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/updates"
)

// ReleaseCheckInterval : une nouvelle version n'a pas besoin d'etre vue a la
// minute, et l'API GitHub limite les appels anonymes a soixante par heure et
// par adresse.
const ReleaseCheckInterval = 6 * time.Hour

// StartReleaseCheck lance la verification periodique des nouvelles versions et
// rend la main immediatement. Elle s'arrete avec le contexte.
//
// Une tache et non un appel pendant la requete : GitHub est un serveur tiers,
// et un ecran ne doit pas l'attendre. L'ecran lit la table, la tache la
// remplit.
func StartReleaseCheck(ctx context.Context, pool *pgxpool.Pool, repository string, log *slog.Logger) {
	client := updates.New()

	go every(ctx, ReleaseCheckInterval, func() {
		checkRelease(ctx, pool, client, repository, log)
	})
}

// every lance `run` tout de suite, puis a chaque intervalle.
func every(ctx context.Context, interval time.Duration, run func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	run()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// checkRelease note la derniere version publiee.
func checkRelease(
	ctx context.Context,
	pool *pgxpool.Pool,
	client *updates.Client,
	repository string,
	log *slog.Logger,
) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	q := db.New(pool)

	release, err := client.LatestRelease(ctx, repository)
	if err != nil {
		// Un passage rate ne fait rien perdre : la derniere version connue
		// reste affichee, et le prochain passage retentera.
		log.Warn("verification des versions echouee", "error", err)
		if saveErr := q.SaveReleaseCheckError(ctx, err.Error()); saveErr != nil {
			log.Warn("enregistrement de l'echec de verification", "error", saveErr)
		}
		return
	}

	if err := q.SaveReleaseCheck(ctx, db.SaveReleaseCheckParams{
		Version:     release.Version,
		Name:        release.Name,
		Url:         release.URL,
		PublishedAt: release.PublishedAt,
	}); err != nil {
		log.Warn("enregistrement de la derniere version", "error", err)
		return
	}

	log.Debug("derniere version publiee", "version", release.Version)
}
