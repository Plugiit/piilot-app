package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ReplyTemplateService est le contrat des reponses types.
type ReplyTemplateService interface {
	List(ctx context.Context) (usecase.ReplyTemplateList, error)
	Create(ctx context.Context, title, body string, actor uuid.UUID) (usecase.ReplyTemplate, error)
	Update(ctx context.Context, id uuid.UUID, title, body string) (usecase.ReplyTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ReplyTemplates porte les reponses types des tickets.
type ReplyTemplates struct {
	svc ReplyTemplateService
}

// NewReplyTemplates construit le handler.
func NewReplyTemplates(svc ReplyTemplateService) *ReplyTemplates { return &ReplyTemplates{svc: svc} }

type replyTemplateRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// List rend les reponses types.
func (h *ReplyTemplates) List(c fiber.Ctx) error {
	out, err := h.svc.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Create ajoute une reponse type.
func (h *ReplyTemplates) Create(c fiber.Ctx) error {
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	var body replyTemplateRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.Create(c.Context(), body.Title, body.Body, actor)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// Update modifie une reponse type.
func (h *ReplyTemplates) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var body replyTemplateRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.Update(c.Context(), id, body.Title, body.Body)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Delete retire une reponse type.
func (h *ReplyTemplates) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
