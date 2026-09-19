package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Reglages par defaut de la purge des jetons de rafraichissement.
const (
	// PurgeInterval : une fois par jour suffit. Rien ne depend de la fraicheur
	// de cette purge, seulement de son existence.
	PurgeInterval = 24 * time.Hour

	// PurgeRetention : delai apres expiration avant suppression definitive. Un
	// jeton expire depuis moins de 7 jours est conserve parce qu'il sert encore
	// a reconnaitre un rejeu ; au-dela, il ne prouve plus rien.
	PurgeRetention = 7 * 24 * time.Hour
)

// StartRefreshTokenPurge lance la purge periodique en tache de fond et rend la
// main immediatement. La goroutine s'arrete quand le contexte est annule,
// c'est-a-dire a l'arret du serveur.
//
// Un job local, pas un cron externe : la tache est breve, sans appel reseau
// sortant, et la faire vivre avec le process evite une piece d'infrastructure
// de plus a deployer et a surveiller.
func StartRefreshTokenPurge(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	go func() {
		ticker := time.NewTicker(PurgeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				purgeRefreshTokens(ctx, pool, log)
			}
		}
	}()
}

// purgeRefreshTokens execute un passage, avec son propre delai de garde pour
// qu'une base lente ne bloque pas la goroutine jusqu'au prochain tick.
func purgeRefreshTokens(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) {
	runCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	retention := pgtype.Interval{
		Microseconds: int64(PurgeRetention / time.Microsecond),
		Valid:        true,
	}

	if err := db.New(pool).DeleteExpiredRefreshTokens(runCtx, retention); err != nil {
		// Une purge ratee n'a aucune consequence fonctionnelle : elle sera
		// retentee au prochain tick. On la signale sans faire echouer quoi que
		// ce soit.
		log.Warn("purge des jetons de rafraichissement echouee", "error", err)
		return
	}

	log.Debug("purge des jetons de rafraichissement effectuee", "retention", PurgeRetention)
}
