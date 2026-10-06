package repository

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/updates"
)

// ReleaseCheckInterval : intervalle par defaut entre deux interrogations de
// GitHub. Un quart d'heure suffit a voir une version le jour de sa sortie, et
// ne coute rien : la requete est conditionnelle, et un « rien de nouveau » ne
// compte pas dans la limite de soixante appels par heure.
const ReleaseCheckInterval = 15 * time.Minute

// releaseCheckTick : la tache se reveille toutes les quinze secondes pour voir
// s'il est temps d'interroger GitHub — ou si un admin vient de le demander,
// qui attend alors sa reponse. Le reveil ne coute qu'une lecture en base.
const releaseCheckTick = 15 * time.Second

// StartReleaseCheck lance la verification periodique des nouvelles versions et
// rend la main immediatement. Elle s'arrete avec le contexte.
//
// Une tache et non un appel pendant la requete : GitHub est un serveur tiers,
// et un ecran ne doit pas l'attendre. L'ecran lit la table, la tache la
// remplit. Une version nouvelle est annoncee une fois aux admins, dans la
// cloche.
func StartReleaseCheck(
	ctx context.Context,
	pool *pgxpool.Pool,
	repository, current string,
	interval time.Duration,
	log *slog.Logger,
) {
	client := updates.New()

	go every(ctx, releaseCheckTick, func() {
		if due(ctx, pool, interval, time.Now(), log) {
			checkRelease(ctx, pool, client, repository, current, log)
		}
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

// due dit s'il faut interroger GitHub : jamais fait, intervalle ecoule, ou
// verification demandee par un admin.
func due(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, now time.Time, log *slog.Logger) bool {
	check, err := db.New(pool).GetReleaseCheck(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return true
	case err != nil:
		log.Warn("lecture de la derniere verification", "error", err)
		return false
	}

	return check.CheckRequestedAt != nil || now.Sub(check.CheckedAt) >= interval
}

// checkRelease note la derniere version publiee, et l'annonce si elle est
// nouvelle.
func checkRelease(
	ctx context.Context,
	pool *pgxpool.Pool,
	client *updates.Client,
	repository, current string,
	log *slog.Logger,
) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	q := db.New(pool)

	previous, err := q.GetReleaseCheck(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Warn("lecture de la derniere verification", "error", err)
		return
	}

	release, etag, err := client.LatestRelease(ctx, repository, previous.Etag)
	switch {
	case errors.Is(err, updates.ErrNotModified):
		if err := q.TouchReleaseCheck(ctx); err != nil {
			log.Warn("enregistrement de la verification", "error", err)
		}
		return
	case err != nil:
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
		Etag:        etag,
	}); err != nil {
		log.Warn("enregistrement de la derniere version", "error", err)
		return
	}

	log.Debug("derniere version publiee", "version", release.Version)

	if updates.Newer(release.Version, current) && release.Version != previous.NotifiedVersion {
		if err := announce(ctx, pool, release); err != nil {
			log.Warn("annonce de la nouvelle version", "error", err)
		}
	}
}

// announce previent dans la cloche ceux qui peuvent installer la version, et
// retient qu'elle l'a ete : une version ne s'annonce qu'une fois.
func announce(ctx context.Context, pool *pgxpool.Pool, release updates.Release) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(pool).WithTx(tx)

	recipients, err := q.ListUpdateRecipients(ctx)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{"version": release.Version, "title": release.Name, "url": release.URL})
	if err != nil {
		return err
	}

	for _, userID := range recipients {
		if _, err := q.CreateNotification(ctx, db.CreateNotificationParams{
			UserID: userID, Kind: "update_available", Payload: payload,
		}); err != nil {
			return err
		}
	}

	if err := q.MarkReleaseNotified(ctx, release.Version); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
