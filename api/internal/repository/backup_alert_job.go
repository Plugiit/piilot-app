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
)

// Rythme de la surveillance des sauvegardes : une par jour est attendue,
// l'heure pres n'a pas d'importance.
const (
	backupWatchInterval = time.Hour
	backupAlertKind     = "backup_stale"
	// Une alerte par jour, pas une par passage : l'absence de sauvegarde est
	// un fait qui dure, pas un evenement qui se repete.
	backupAlertEvery = 24 * time.Hour
)

// StartBackupWatch previent les admins quand la derniere sauvegarde reussie
// date de plus de `staleAfter`, ou qu'il n'y en a jamais eu. Rien ne sort
// d'ici : une notification dans la cloche, lue en base.
func StartBackupWatch(ctx context.Context, pool *pgxpool.Pool, staleAfter time.Duration, log *slog.Logger) {
	if staleAfter <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(backupWatchInterval)
		defer ticker.Stop()

		// Au demarrage, un peu plus tard : une instance qui vient de naitre n'a
		// pas de sauvegarde, et ses admins le savent.
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Minute):
			watchBackups(ctx, pool, staleAfter, log)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				watchBackups(ctx, pool, staleAfter, log)
			}
		}
	}()
}

func watchBackups(ctx context.Context, pool *pgxpool.Pool, staleAfter time.Duration, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	q := db.New(pool)
	if err := q.AbandonStaleBackups(ctx); err != nil {
		log.Warn("sauvegardes : cloture des interrompues", "error", err)
	}

	now := time.Now()
	var since *time.Time
	last, err := q.GetLastSuccessfulBackup(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		log.Warn("sauvegardes : lecture", "error", err)
		return
	default:
		if now.Sub(last.StartedAt) <= staleAfter {
			return
		}
		since = &last.StartedAt
	}

	lastAlert, err := q.LastNotificationOfKind(ctx, backupAlertKind)
	if err != nil {
		log.Warn("sauvegardes : derniere alerte", "error", err)
		return
	}
	if now.Sub(lastAlert) < backupAlertEvery {
		return
	}

	recipients, err := q.ListUpdateRecipients(ctx)
	if err != nil {
		log.Warn("sauvegardes : destinataires", "error", err)
		return
	}

	payload, err := json.Marshal(map[string]any{"last_success_at": since, "stale_after": staleAfter.String()})
	if err != nil {
		return
	}
	for _, userID := range recipients {
		if _, err := q.CreateNotification(ctx, db.CreateNotificationParams{
			UserID: userID, Kind: backupAlertKind, Payload: payload,
		}); err != nil {
			log.Warn("sauvegardes : notification", "error", err)
			return
		}
	}

	log.Warn("aucune sauvegarde recente", "derniere", since, "destinataires", len(recipients))
}
