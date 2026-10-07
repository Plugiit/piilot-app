package usecase

import (
	"context"
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

// ReplyTemplate est une reponse type : un titre pour la retrouver, un texte a
// inserer dans la reponse a un ticket. Les variables {prenom}, {numero},
// {sujet} et {projet} sont remplacees a l'insertion, cote ecran.
type ReplyTemplate struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReplyTemplateList est la liste complete : quelques dizaines au plus.
type ReplyTemplateList struct {
	Items []ReplyTemplate `json:"items"`
}

// ReplyTemplateService gere les reponses types.
type ReplyTemplateService struct {
	q *db.Queries
}

// NewReplyTemplateService construit le service.
func NewReplyTemplateService(pool *pgxpool.Pool) *ReplyTemplateService {
	return &ReplyTemplateService{q: db.New(pool)}
}

func replyTemplateOf(r db.TicketReplyTemplate) ReplyTemplate {
	return ReplyTemplate{ID: r.ID, Title: r.Title, Body: r.Body, UpdatedAt: r.UpdatedAt}
}

func validateReplyTemplate(title, body string) (string, string, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	details := map[string]any{}
	if title == "" {
		details["title"] = "Le titre est requis"
	} else if len([]rune(title)) > 120 {
		details["title"] = "120 caractères au plus"
	}
	if body == "" {
		details["body"] = "Le texte est requis"
	}
	if len(details) > 0 {
		return "", "", domain.ErrValidation.WithDetails(details)
	}
	return title, body, nil
}

// List rend toutes les reponses types, par titre.
func (s *ReplyTemplateService) List(ctx context.Context) (ReplyTemplateList, error) {
	rows, err := s.q.ListReplyTemplates(ctx)
	if err != nil {
		return ReplyTemplateList{}, fmt.Errorf("lecture des reponses types : %w", err)
	}
	out := ReplyTemplateList{Items: make([]ReplyTemplate, 0, len(rows))}
	for _, r := range rows {
		out.Items = append(out.Items, replyTemplateOf(r))
	}
	return out, nil
}

// Create ajoute une reponse type.
func (s *ReplyTemplateService) Create(ctx context.Context, title, body string, actor uuid.UUID) (ReplyTemplate, error) {
	title, body, err := validateReplyTemplate(title, body)
	if err != nil {
		return ReplyTemplate{}, err
	}
	row, err := s.q.CreateReplyTemplate(ctx, db.CreateReplyTemplateParams{Title: title, Body: body, CreatedBy: &actor})
	if err != nil {
		return ReplyTemplate{}, fmt.Errorf("creation de la reponse type : %w", err)
	}
	return replyTemplateOf(row), nil
}

// Update modifie une reponse type.
func (s *ReplyTemplateService) Update(ctx context.Context, id uuid.UUID, title, body string) (ReplyTemplate, error) {
	title, body, err := validateReplyTemplate(title, body)
	if err != nil {
		return ReplyTemplate{}, err
	}
	row, err := s.q.UpdateReplyTemplate(ctx, db.UpdateReplyTemplateParams{ID: id, Title: title, Body: body})
	if errors.Is(err, pgx.ErrNoRows) {
		return ReplyTemplate{}, domain.ErrNotFound
	}
	if err != nil {
		return ReplyTemplate{}, fmt.Errorf("modification de la reponse type : %w", err)
	}
	return replyTemplateOf(row), nil
}

// Delete retire une reponse type.
func (s *ReplyTemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteReplyTemplate(ctx, id)
	if err != nil {
		return fmt.Errorf("suppression de la reponse type : %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
