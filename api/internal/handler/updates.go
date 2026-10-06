package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// UpdateService est le contrat dont la mise a jour a besoin.
type UpdateService interface {
	Status(ctx context.Context) (usecase.UpdateStatus, error)
	Request(ctx context.Context, actor uuid.UUID) (usecase.UpdateStatus, error)
	RequestCheck(ctx context.Context) (usecase.UpdateStatus, error)
}

// Updates porte la mise a jour de l'application depuis l'interface.
type Updates struct {
	svc UpdateService
}

// NewUpdates construit le handler.
func NewUpdates(svc UpdateService) *Updates {
	return &Updates{svc: svc}
}

// Status sert l'etat de la mise a jour : version en cours, derniere version
// publiee, derniere demande.
func (h *Updates) Status(c fiber.Ctx) error {
	status, err := h.svc.Status(c.Context())
	if err != nil {
		return err
	}

	return c.JSON(status)
}

// Request lance la mise a jour vers la derniere version. 202 : la demande est
// enregistree, l'updater la prend dans les secondes qui suivent.
func (h *Updates) Request(c fiber.Ctx) error {
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	status, err := h.svc.Request(c.Context(), actor)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(status)
}

// Check demande une verification immediate des versions. 202 : la tache de
// fond la fait dans les quinze secondes, l'ecran relit l'etat.
func (h *Updates) Check(c fiber.Ctx) error {
	status, err := h.svc.RequestCheck(c.Context())
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(status)
}
