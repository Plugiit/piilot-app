package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Reglages de l'envoi des e-mails.
const (
	// MailInterval : un lien d'invitation ou de reinitialisation est attendu
	// par quelqu'un devant son ecran ; quelques secondes de delai suffisent.
	MailInterval = 5 * time.Second
	// MailBatch borne un passage : chaque e-mail coute un aller-retour SMTP.
	MailBatch = 10
	// MailMaxAttempts : au-dela, l'e-mail est abandonne. Six essais espaces
	// de 1, 2, 4, 8 et 16 minutes couvrent une panne d'une demi-heure.
	MailMaxAttempts = 6
)

// StartMailOutbox lance l'envoi des e-mails de la file et rend la main
// immediatement. La goroutine s'arrete avec le contexte.
func StartMailOutbox(ctx context.Context, pool *pgxpool.Pool, sender mail.SMTP, log *slog.Logger) {
	go every(ctx, MailInterval, func() {
		for range MailBatch {
			if !sendNext(ctx, pool, sender, log) {
				return
			}
		}
	})
}

// sendNext envoie le prochain e-mail du, s'il y en a un, et dit s'il faut
// continuer.
//
// L'e-mail est pris sous verrou, dans une transaction : deux instances qui
// tournent ensemble pendant un deploiement n'envoient pas deux fois le meme.
func sendNext(ctx context.Context, pool *pgxpool.Pool, sender mail.SMTP, log *slog.Logger) bool {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Warn("envoi des e-mails : ouverture de transaction", "error", err)
		return false
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	email, err := q.ClaimDueEmail(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	if err != nil {
		log.Warn("envoi des e-mails : lecture", "error", err)
		return false
	}

	sendErr := sender.Send(ctx, mail.Message{
		Kind: email.Kind, To: email.ToAddress, Subject: email.Subject,
		Text: email.TextBody, HTML: email.HtmlBody,
		ReplyTo: email.ReplyTo, MessageID: email.MessageID, InReplyTo: email.InReplyTo,
	})

	switch {
	case sendErr == nil:
		err = q.MarkEmailSent(ctx, email.ID)
		log.Info("e-mail envoye", "kind", email.Kind, "id", email.ID)
	case int(email.Attempts)+1 >= MailMaxAttempts:
		err = q.MarkEmailFailed(ctx, db.MarkEmailFailedParams{ID: email.ID, LastError: sendErr.Error()})
		log.Error("e-mail abandonne", "kind", email.Kind, "id", email.ID, "error", sendErr)
	default:
		// 1, 2, 4, 8, 16 minutes : un serveur SMTP en panne n'est pas
		// sollicite toutes les cinq secondes.
		delay := time.Minute << email.Attempts
		err = q.MarkEmailRetry(ctx, db.MarkEmailRetryParams{
			ID: email.ID, NextAttemptAt: time.Now().Add(delay), LastError: sendErr.Error(),
		})
		log.Warn("e-mail non envoye, nouvel essai prevu", "kind", email.Kind, "id", email.ID, "dans", delay, "error", sendErr)
	}
	if err != nil {
		log.Warn("envoi des e-mails : enregistrement du resultat", "error", err)
		return false
	}

	if err := tx.Commit(ctx); err != nil {
		log.Warn("envoi des e-mails : validation", "error", err)
		return false
	}

	return true
}
