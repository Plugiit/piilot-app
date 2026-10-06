package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
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

	NotifyTicketCreated       = "ticket_created"
	NotifyTicketAssigned      = "ticket_assigned"
	NotifyTicketReplied       = "ticket_replied"
	NotifyTicketStatusChanged = "ticket_status_changed"

	NotifyDeliverableValidated = "deliverable_validated"
	NotifyDeliverableFeedback  = "deliverable_feedback"

	// Ecrite par la tache de verification des versions, pour ceux qui peuvent
	// installer la mise a jour.
	NotifyUpdateAvailable = "update_available"
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
	// Ticket ou livrable vise, pour les notifications qui en parlent : l'ecran
	// mene a la fiche du ticket, ou a la liste des livrables.
	TicketID      *uuid.UUID `json:"ticket_id"`
	DeliverableID *uuid.UUID `json:"deliverable_id"`
	Actor         *Person    `json:"actor"`
	ReadAt        *time.Time `json:"read_at"`
	CreatedAt     time.Time  `json:"created_at"`
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
			ID:            row.ID,
			Kind:          row.Kind,
			Payload:       payload,
			TaskID:        row.TaskID,
			ProjectID:     row.ProjectID,
			TicketID:      row.TicketID,
			DeliverableID: row.DeliverableID,
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

	return deliver(ctx, q, bus, notice{
		Kind:       event.Kind,
		ActorID:    event.ActorID,
		Payload:    event.Payload,
		TaskID:     &event.TaskID,
		ProjectID:  &event.ProjectID,
		Recipients: recipients,
	})
}

// notice est une notification prete a ecrire, quel que soit l'objet dont elle
// parle.
type notice struct {
	Kind          string
	ActorID       uuid.UUID
	Payload       map[string]any
	TaskID        *uuid.UUID
	ProjectID     *uuid.UUID
	TicketID      *uuid.UUID
	DeliverableID *uuid.UUID
	Recipients    []uuid.UUID
}

// deliver ecrit une ligne par destinataire et reveille leurs flux.
//
// La diffusion part tout de suite, avant la fin de la transaction. Si
// celle-ci echoue apres coup, un navigateur aura ete reveille pour rien : il
// rechargera et ne verra rien de neuf. L'inverse — diffuser apres le commit —
// demanderait de porter la liste des messages jusqu'apres la transaction, a
// travers chaque point d'appel, pour eviter un cas qui ne coute qu'une requete.
func deliver(ctx context.Context, q *db.Queries, bus Bus, n notice) error {
	if len(n.Recipients) == 0 {
		return nil
	}

	payload, err := json.Marshal(n.Payload)
	if err != nil {
		return fmt.Errorf("encodage de la notification : %w", err)
	}

	seen := make(map[uuid.UUID]bool, len(n.Recipients))

	for _, userID := range n.Recipients {
		// L'auteur n'est jamais prevenu de son propre geste, y compris quand
		// les destinataires sont imposes. Ni prevenu deux fois du meme.
		if userID == n.ActorID || seen[userID] {
			continue
		}
		seen[userID] = true

		row, err := q.CreateNotification(ctx, db.CreateNotificationParams{
			UserID:        userID,
			ActorID:       &n.ActorID,
			Kind:          n.Kind,
			Payload:       payload,
			TaskID:        n.TaskID,
			ProjectID:     n.ProjectID,
			TicketID:      n.TicketID,
			DeliverableID: n.DeliverableID,
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
