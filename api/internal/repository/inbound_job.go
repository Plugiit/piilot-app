package repository

import (
	"context"
	"log/slog"
	"time"
)

// Rythme de l'e-mail entrant.
const (
	// inboundTick : le rangement tourne toutes les dix secondes — un e-mail
	// pousse par webhook rejoint son ticket aussitot.
	inboundTick = 10 * time.Second
	// inboundPollEvery : la boite IMAP est relevee chaque minute, plus tot
	// quand un admin le demande.
	inboundPollEvery = time.Minute
)

// InboundWorker est ce que la tache attend du service de l'e-mail entrant.
type InboundWorker interface {
	PollDue(ctx context.Context, every time.Duration) bool
	PollIMAP(ctx context.Context) error
	ProcessPending(ctx context.Context)
}

// StartInboundMail releve la boite et range les e-mails recus. Tout se passe
// ici, hors de toute requete HTTP : c'est le seul endroit ou Piilot parle a
// un serveur IMAP.
func StartInboundMail(ctx context.Context, w InboundWorker, log *slog.Logger) {
	go every(ctx, inboundTick, func() {
		pollCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		if w.PollDue(pollCtx, inboundPollEvery) {
			if err := w.PollIMAP(pollCtx); err != nil {
				log.Warn("releve IMAP", "error", err)
			}
		}
		w.ProcessPending(pollCtx)
	})
}

// AuditPurger est ce que la purge attend du journal d'audit.
type AuditPurger interface {
	Purge(ctx context.Context) (int64, error)
}

// StartAuditPurge efface chaque jour les lignes du journal d'audit plus
// vieilles que la retention.
func StartAuditPurge(ctx context.Context, p AuditPurger, log *slog.Logger) {
	go every(ctx, 24*time.Hour, func() {
		purgeCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		n, err := p.Purge(purgeCtx)
		if err != nil {
			log.Warn("purge du journal d'audit", "error", err)
			return
		}
		if n > 0 {
			log.Info("journal d'audit purge", "lignes", n)
		}
	})
}
