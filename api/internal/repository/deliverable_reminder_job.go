package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	mailer "github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// DeliverableReminderInterval : rythme des passages. Une relance se compte
// en jours, l'heure pres n'a pas d'importance.
const DeliverableReminderInterval = time.Hour

// StartDeliverableReminders relance par e-mail les clients dont un livrable
// attend une reponse depuis plus de `after`. Une fois par version, jamais
// plus : c'etait la corvee du lundi matin, elle n'a plus lieu d'etre.
//
// Les e-mails partent par la file d'envoi, comme les autres : rien ne sort
// d'ici vers un serveur tiers.
func StartDeliverableReminders(ctx context.Context, pool *pgxpool.Pool, clientURL string, after time.Duration, log *slog.Logger) {
	if after <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(DeliverableReminderInterval)
		defer ticker.Stop()

		remindDeliverables(ctx, pool, clientURL, after, log)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				remindDeliverables(ctx, pool, clientURL, after, log)
			}
		}
	}()
}

func remindDeliverables(ctx context.Context, pool *pgxpool.Pool, clientURL string, after time.Duration, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	q := db.New(pool)
	rows, err := q.ListVersionsToRemind(ctx, after.Seconds())
	if err != nil {
		log.Warn("relance des livrables : lecture", "error", err)
		return
	}

	for _, row := range rows {
		recipients, err := q.ListPortalRecipientsOfProject(ctx, row.ProjectID)
		if err != nil {
			log.Warn("relance des livrables : comptes du portail", "error", err)
			return
		}

		days := int(time.Since(row.SubmittedAt).Hours() / 24)
		if days < 1 {
			days = 1
		}
		url := fmt.Sprintf("%s/client/livrables/%s", clientURL, row.DeliverableID)

		for _, r := range recipients {
			msg, err := mailer.DeliverableReminder(r.Email, r.Firstname, row.ProjectName, row.Title, int(row.Numero), days, url)
			if err != nil {
				log.Warn("relance des livrables : composition", "error", err)
				return
			}
			if err := q.EnqueueEmail(ctx, db.EnqueueEmailParams{
				Kind: msg.Kind, ToAddress: msg.To, Subject: msg.Subject, TextBody: msg.Text, HtmlBody: msg.HTML,
			}); err != nil {
				log.Warn("relance des livrables : mise en file", "error", err)
				return
			}
		}

		// Marquee meme sans destinataire : un projet sans compte de portail
		// n'a personne a relancer, et le repasser chaque heure n'y changerait
		// rien.
		if err := q.MarkVersionReminded(ctx, row.VersionID); err != nil {
			log.Warn("relance des livrables : marquage", "error", err)
			return
		}

		if len(recipients) > 0 {
			log.Info("livrable relance", "deliverable", row.DeliverableID, "version", row.Numero, "destinataires", len(recipients))
		}
	}
}
