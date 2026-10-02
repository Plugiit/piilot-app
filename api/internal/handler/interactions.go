package handler

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// InteractionService est le contrat du journal client.
type InteractionService interface {
	List(ctx context.Context, f usecase.InteractionFilters) (usecase.InteractionFeed, error)
	Create(ctx context.Context, clientID uuid.UUID, in usecase.InteractionInput) (uuid.UUID, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Interactions porte le journal de la relation client.
type Interactions struct {
	svc InteractionService
}

// NewInteractions construit le handler.
func NewInteractions(svc InteractionService) *Interactions {
	return &Interactions{svc: svc}
}

// List sert l'ecran « Interactions », et le journal d'une fiche client avec
// `client_id`.
func (h *Interactions) List(c fiber.Ctx) error {
	f := usecase.InteractionFilters{Source: strings.TrimSpace(c.Query("source"))}

	if raw := strings.TrimSpace(c.Query("client_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"client_id": "Identifiant invalide"})
		}
		f.ClientID = &id
	}

	if raw := strings.TrimSpace(c.Query("before")); raw != "" {
		before, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"before": "Date attendue au format RFC 3339"})
		}
		f.Before = &before
	}

	feed, err := h.svc.List(c.Context(), f)
	if err != nil {
		return err
	}

	return c.JSON(feed)
}

// Create inscrit une note, un appel, un rendez-vous ou un e-mail.
func (h *Interactions) Create(c fiber.Ctx) error {
	clientID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body struct {
		Kind       string  `json:"kind"`
		Body       string  `json:"body"`
		OccurredAt *string `json:"occurred_at"`
		ProjectID  *string `json:"project_id"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	in := usecase.InteractionInput{Kind: body.Kind, Body: body.Body, AuthorID: actor}

	if body.OccurredAt != nil && strings.TrimSpace(*body.OccurredAt) != "" {
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.OccurredAt))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"occurred_at": "Date attendue au format RFC 3339"})
		}
		in.OccurredAt = &at
	}

	if body.ProjectID != nil && strings.TrimSpace(*body.ProjectID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*body.ProjectID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"project_id": "Identifiant invalide"})
		}
		in.ProjectID = &id
	}

	id, err := h.svc.Create(c.Context(), clientID, in)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

// Delete efface une saisie.
func (h *Interactions) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
