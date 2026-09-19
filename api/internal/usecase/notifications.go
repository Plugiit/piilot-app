package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/plugiit-api-go/internal/repository/db"
)

// Bus porte les notifications jusqu'aux flux ouverts.
//
// Declare ici, chez le consommateur : le service n'a pas besoin de savoir que
// c'est Redis derriere, et les tests n'ont pas besoin de Redis.
type Bus interface {
	Publish(ctx context.Context, userID uuid.UUID, payload []byte) error
}

// Genres de notification. Le meme jeu que la contrainte posee en base — les
// deux doivent bouger ensemble.
const (
	NotifyTaskCreated       = "task_created"
	NotifyTaskStatusChanged = "task_status_changed"
	NotifyTaskAssigned      = "task_assigned"
	NotifyTaskUnassigned    = "task_unassigned"
	NotifyTaskCommented     = "task_commented"
	NotifyTaskDueChanged    = "task_due_changed"
	NotifyProjectCreated    = "project_created"
)

// Notification est une ligne du panneau.
//
// Le serveur dit ce qui s'est passe et avec quoi ; l'ecran ecrit la phrase
// dans sa langue, comme pour le journal d'une tache.
type Notification struct {
	ID        uuid.UUID      `json:"id"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
	TaskID    *uuid.UUID     `json:"task_id"`
	ProjectID *uuid.UUID     `json:"project_id"`
	Actor     *Person        `json:"actor"`
	ReadAt    *time.Time     `json:"read_at"`
	CreatedAt time.Time      `json:"created_at"`
}

// NotificationFeed est le contenu du panneau a son ouverture.
type NotificationFeed struct {
	Items  []Notification `json:"items"`
	Unread int64          `json:"unread"`
	// Before sert a demander la suite : la date de la derniere ligne rendue.
	// Nul quand il n'y a plus rien apres.
	Before *time.Time `json:"before"`
}

// notificationPageSize borne une page du panneau.
const notificationPageSize = 30

// NotificationService lit et ecrit les notifications.
type NotificationService struct {
	q   *db.Queries
	bus Bus
}

func NewNotificationService(pool *pgxpool.Pool, bus Bus) *NotificationService {
	return &NotificationService{q: db.New(pool), bus: bus}
}

// Feed rend les notifications d'une personne, les plus recentes d'abord.
func (s *NotificationService) Feed(
	ctx context.Context,
	userID uuid.UUID,
	before *time.Time,
) (NotificationFeed, error) {
	rows, err := s.q.ListNotifications(ctx, db.ListNotificationsParams{
		UserID:   userID,
		Before:   before,
		PageSize: notificationPageSize,
	})
	if err != nil {
		return NotificationFeed{}, fmt.Errorf("lecture des notifications : %w", err)
	}

	unread, err := s.q.CountUnreadNotifications(ctx, userID)
	if err != nil {
		return NotificationFeed{}, fmt.Errorf("comptage des non-lues : %w", err)
	}

	items := make([]Notification, 0, len(rows))
	for _, row := range rows {
		payload := map[string]any{}
		// Un payload illisible ne doit pas faire echouer l'ouverture du
		// panneau : la ligne passe avec un contenu vide.
		_ = json.Unmarshal(row.Payload, &payload)

		items = append(items, Notification{
			ID:        row.ID,
			Kind:      row.Kind,
			Payload:   payload,
			TaskID:    row.TaskID,
			ProjectID: row.ProjectID,
			Actor: personFromNullable(
				row.ActorID, row.ActorFirstname, row.ActorLastname, row.ActorAvatarUrl,
			),
			ReadAt:    row.ReadAt,
			CreatedAt: row.CreatedAt,
		})
	}

	feed := NotificationFeed{Items: items, Unread: unread}

	// Un curseur seulement quand la page est pleine : plus courte, elle est la
	// derniere, et rendre une date ferait demander une suite vide.
	if len(items) == notificationPageSize {
		feed.Before = &items[len(items)-1].CreatedAt
	}

	return feed, nil
}

// MarkRead marque une notification comme lue.
func (s *NotificationService) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	if err := s.q.MarkNotificationRead(ctx, db.MarkNotificationReadParams{
		ID:     id,
		UserID: userID,
	}); err != nil {
		return fmt.Errorf("marquage de la notification : %w", err)
	}

	return nil
}

// MarkAllRead vide le compteur d'une personne.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if err := s.q.MarkAllNotificationsRead(ctx, userID); err != nil {
		return fmt.Errorf("marquage des notifications : %w", err)
	}

	return nil
}

// TaskEvent decrit un geste pose sur une tache, pret a etre notifie.
type TaskEvent struct {
	TaskID    uuid.UUID
	ProjectID uuid.UUID
	ActorID   uuid.UUID
	Kind      string
	Payload   map[string]any
	// Recipients force les destinataires. Vide, ils sont deduits : les
	// administrateurs et les personnes affectees a la tache. Utile pour
	// prevenir quelqu'un qui vient d'etre retire, et qui n'est donc plus
	// affecte au moment ou l'on cherche qui prevenir.
	Recipients []uuid.UUID
}

// notifyTask ecrit une notification par destinataire et la diffuse.
//
// Prend les requetes en parametre pour s'executer dans la transaction de
// l'appelant : la notification et le geste qu'elle annonce tombent ou
// aboutissent ensemble.
//
// La diffusion, elle, part tout de suite, avant la fin de la transaction. Si
// celle-ci echoue apres coup, un navigateur aura ete reveille pour rien : il
// rechargera et ne verra rien de neuf. L'inverse — diffuser apres le commit —
// demanderait de porter la liste des messages jusqu'apres la transaction, a
// travers huit points d'appel, pour eviter un cas qui ne coute qu'une requete.
func notifyTask(
	ctx context.Context,
	q *db.Queries,
	bus Bus,
	event TaskEvent,
) error {
	recipients := event.Recipients

	if len(recipients) == 0 {
		found, err := q.ListNotificationRecipients(ctx, db.ListNotificationRecipientsParams{
			TaskID:  &event.TaskID,
			ActorID: event.ActorID,
		})
		if err != nil {
			return fmt.Errorf("recherche des destinataires : %w", err)
		}

		recipients = found
	}

	if len(recipients) == 0 {
		return nil
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("encodage de la notification : %w", err)
	}

	for _, userID := range recipients {
		// L'auteur n'est jamais prevenu de son propre geste, y compris quand
		// les destinataires sont imposes.
		if userID == event.ActorID {
			continue
		}

		row, err := q.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:    userID,
			ActorID:   &event.ActorID,
			Kind:      event.Kind,
			Payload:   payload,
			TaskID:    &event.TaskID,
			ProjectID: &event.ProjectID,
		})
		if err != nil {
			return fmt.Errorf("ecriture de la notification : %w", err)
		}

		if bus == nil {
			continue
		}

		// Le flux ne transporte que l'identifiant et le genre : l'ecran
		// recharge sa liste, qui reste la seule source de verite. Y recopier la
		// notification entiere ferait deux formats a tenir.
		message, err := json.Marshal(map[string]any{
			"id":   row.ID,
			"kind": row.Kind,
		})
		if err != nil {
			return fmt.Errorf("encodage du message : %w", err)
		}

		// Une diffusion qui echoue ne doit pas annuler le geste : la
		// notification est en base, elle sera lue au prochain chargement.
		_ = bus.Publish(ctx, userID, message)
	}

	return nil
}
