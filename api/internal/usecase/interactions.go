package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Genres d'interaction. Les quatre premiers se saisissent a la main ; les
// evenements s'ecrivent seuls. Le meme jeu que la contrainte en base.
const (
	InteractionNote                 = "note"
	InteractionCall                 = "call"
	InteractionMeeting              = "meeting"
	InteractionEmail                = "email"
	InteractionProjectCreated       = "project_created"
	InteractionDeliverableValidated = "deliverable_validated"
	InteractionTicketOpened         = "ticket_opened"
)

var manualInteractions = map[string]bool{
	InteractionNote: true, InteractionCall: true, InteractionMeeting: true, InteractionEmail: true,
}

var interactionKinds = map[string]bool{
	InteractionNote: true, InteractionCall: true, InteractionMeeting: true, InteractionEmail: true,
	InteractionProjectCreated: true, InteractionDeliverableValidated: true, InteractionTicketOpened: true,
}

// interactionPageSize borne une page du journal.
const interactionPageSize = 30

// Interaction est une ligne du journal.
type Interaction struct {
	ID         uuid.UUID       `json:"id"`
	Kind       string          `json:"kind"`
	Body       string          `json:"body"`
	OccurredAt time.Time       `json:"occurred_at"`
	Client     DeliverableRef  `json:"client"`
	Project    *DeliverableRef `json:"project"`
	Author     *Person         `json:"author"`
	// Ce que dit un evenement : titre, numero de ticket. Vide pour une saisie.
	Payload map[string]any `json:"payload"`
	// Vrai pour une saisie a la main : elle peut etre effacee.
	Manual bool `json:"manual"`
}

// InteractionFeed est une page du journal.
type InteractionFeed struct {
	Items []Interaction `json:"items"`
	// Curseur de la page suivante ; nul quand il n'y a plus rien.
	Before *time.Time `json:"before"`
}

// InteractionFilters porte ce que l'ecran peut demander.
type InteractionFilters struct {
	ClientID *uuid.UUID
	// "manual" pour les saisies, "events" pour les evenements, vide pour tout.
	Source string
	Before *time.Time
}

// InteractionInput est une saisie a la main.
type InteractionInput struct {
	Kind       string
	Body       string
	OccurredAt *time.Time
	ProjectID  *uuid.UUID
	AuthorID   uuid.UUID
}

// InteractionService tient le journal de la relation client.
type InteractionService struct {
	q *db.Queries
}

func NewInteractionService(pool *pgxpool.Pool) *InteractionService {
	return &InteractionService{q: db.New(pool)}
}

// List rend une page du journal, du plus recent au plus ancien.
func (s *InteractionService) List(ctx context.Context, f InteractionFilters) (InteractionFeed, error) {
	var kinds []string
	switch f.Source {
	case "manual":
		kinds = []string{InteractionNote, InteractionCall, InteractionMeeting, InteractionEmail}
	case "events":
		kinds = []string{InteractionProjectCreated, InteractionDeliverableValidated, InteractionTicketOpened}
	case "":
	default:
		if !interactionKinds[f.Source] {
			return InteractionFeed{}, domain.ErrValidation.WithDetails(map[string]any{"source": "Filtre inconnu"})
		}
		kinds = []string{f.Source}
	}

	rows, err := s.q.ListInteractions(ctx, db.ListInteractionsParams{
		ClientID: f.ClientID,
		Kinds:    kinds,
		Before:   f.Before,
		PageSize: interactionPageSize,
	})
	if err != nil {
		return InteractionFeed{}, fmt.Errorf("lecture des interactions : %w", err)
	}

	items := make([]Interaction, 0, len(rows))
	for _, row := range rows {
		payload := map[string]any{}
		_ = json.Unmarshal(row.Payload, &payload)

		var project *DeliverableRef
		if row.ProjectID != nil && row.ProjectName != nil {
			project = &DeliverableRef{ID: *row.ProjectID, Name: *row.ProjectName}
		}

		items = append(items, Interaction{
			ID:         row.ID,
			Kind:       row.Kind,
			Body:       row.Body,
			OccurredAt: row.OccurredAt,
			Client:     DeliverableRef{ID: row.ClientID, Name: row.ClientName},
			Project:    project,
			Author:     personFromNullable(row.AuthorID, row.AuthorFirstname, row.AuthorLastname, row.AuthorAvatarUrl),
			Payload:    payload,
			Manual:     manualInteractions[row.Kind],
		})
	}

	feed := InteractionFeed{Items: items}
	if len(items) == interactionPageSize {
		feed.Before = &items[len(items)-1].OccurredAt
	}

	return feed, nil
}

// Create inscrit une saisie au journal d'un client.
func (s *InteractionService) Create(ctx context.Context, clientID uuid.UUID, in InteractionInput) (uuid.UUID, error) {
	body := strings.TrimSpace(in.Body)
	details := map[string]any{}
	if !manualInteractions[in.Kind] {
		details["kind"] = "Genre attendu : note, appel, rendez-vous ou e-mail"
	}
	if body == "" {
		details["body"] = "Le contenu est requis"
	}
	if len(details) > 0 {
		return uuid.Nil, domain.ErrValidation.WithDetails(details)
	}

	occurred := time.Now()
	if in.OccurredAt != nil {
		if in.OccurredAt.After(occurred.Add(24 * time.Hour)) {
			return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{
				"occurred_at": "Une interaction se note une fois passée",
			})
		}
		occurred = *in.OccurredAt
	}

	id, err := s.q.CreateInteraction(ctx, db.CreateInteractionParams{
		ClientID:   clientID,
		Kind:       in.Kind,
		Body:       body,
		OccurredAt: occurred,
		AuthorID:   &in.AuthorID,
		ProjectID:  in.ProjectID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return uuid.Nil, domain.ErrNotFound
		}

		return uuid.Nil, fmt.Errorf("inscription de l'interaction : %w", err)
	}

	return id, nil
}

// Delete efface une saisie. Un evenement ne s'efface pas : c'est une trace.
func (s *InteractionService) Delete(ctx context.Context, id uuid.UUID) error {
	row, err := s.q.GetInteraction(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}

		return fmt.Errorf("lecture de l'interaction : %w", err)
	}
	if !manualInteractions[row.Kind] {
		return domain.ErrValidation.WithDetails(map[string]any{
			"id": "Un événement automatique ne s'efface pas",
		})
	}

	if _, err := s.q.DeleteInteraction(ctx, id); err != nil {
		return fmt.Errorf("suppression de l'interaction : %w", err)
	}

	return nil
}

// recordProjectEvent inscrit un evenement au journal du client d'un projet,
// dans la transaction de l'appelant : le geste et sa trace aboutissent
// ensemble.
func recordProjectEvent(
	ctx context.Context,
	q *db.Queries,
	projectID uuid.UUID,
	kind string,
	authorID *uuid.UUID,
	payload map[string]any,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encodage de l'evenement : %w", err)
	}

	if err := q.RecordProjectEvent(ctx, db.RecordProjectEventParams{
		Kind: kind, AuthorID: authorID, Payload: encoded, ProjectID: projectID,
	}); err != nil {
		return fmt.Errorf("journal du client : %w", err)
	}

	return nil
}
