package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	mailer "github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// siteAlert est ce qui menace un site : les admins l'apprennent par la cloche
// et par e-mail, car une alerte que personne ne vient lire ne protege rien.
type siteAlert struct {
	Kind      string
	ProjectID uuid.UUID
	Payload   map[string]any
	Subject   string
	Message   string
}

// sendSiteAlert ecrit la notification et l'e-mail avec le meme q : dans une
// transaction, l'alerte et l'etat qu'elle annonce sont ecrits ensemble. L'e-mail
// passe par la file d'envoi : aucun appel exterieur ici.
func sendSiteAlert(ctx context.Context, q *db.Queries, canSend bool, baseURL string, a siteAlert) error {
	body, err := json.Marshal(a.Payload)
	if err != nil {
		return fmt.Errorf("alerte %s : %w", a.Kind, err)
	}
	admins, err := q.ListReportAdmins(ctx)
	if err != nil {
		return fmt.Errorf("destinataires de l'alerte : %w", err)
	}
	url := fmt.Sprintf("%s/pm/projets/%s/parametres/maintenance", baseURL, a.ProjectID)
	for _, admin := range admins {
		if _, err := q.CreateNotification(ctx, db.CreateNotificationParams{UserID: admin.ID, Kind: a.Kind, Payload: body, ProjectID: &a.ProjectID}); err != nil {
			return fmt.Errorf("alerte %s : %w", a.Kind, err)
		}
		if !canSend {
			continue
		}
		msg, err := mailer.SiteAlert(admin.Email, admin.Firstname, a.Subject, a.Message, url)
		if err != nil {
			return fmt.Errorf("e-mail de l'alerte %s : %w", a.Kind, err)
		}
		if err := enqueue(ctx, q, msg); err != nil {
			return err
		}
	}
	return nil
}

// inTx execute run dans une transaction.
func inTx(ctx context.Context, pool *pgxpool.Pool, base *db.Queries, run func(q *db.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := run(base.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction : %w", err)
	}
	return nil
}
