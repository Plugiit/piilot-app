package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// MyWorkService est le contrat de la page « Mon travail ».
type MyWorkService interface {
	Get(ctx context.Context, userID uuid.UUID) (usecase.MyWork, error)
}

// MyWork porte la page « Mon travail ».
type MyWork struct {
	svc MyWorkService
}

// NewMyWork construit le handler.
func NewMyWork(svc MyWorkService) *MyWork {
	return &MyWork{svc: svc}
}

// Get sert le travail de la personne connectee. Aucun parametre de compte :
// la page ne montre jamais que le sien.
func (h *MyWork) Get(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	work, err := h.svc.Get(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(work)
}
