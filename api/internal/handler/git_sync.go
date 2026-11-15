package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// GitSyncService est ce que le handler attend du rattrapage Git.
type GitSyncService interface {
	Status(ctx context.Context, projectID uuid.UUID) (usecase.GitSyncView, error)
	RequestSync(ctx context.Context, projectID uuid.UUID) (usecase.GitSyncView, error)
}

// GitSync porte le rattrapage des pull requests d'un projet.
type GitSync struct {
	svc GitSyncService
}

// NewGitSync construit le handler.
func NewGitSync(svc GitSyncService) *GitSync { return &GitSync{svc: svc} }

// Status rend ou en est la synchronisation d'un projet.
func (h *GitSync) Status(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	out, err := h.svc.Status(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Sync demande une synchronisation : la tache de fond la fait dans la minute.
func (h *GitSync) Sync(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	out, err := h.svc.RequestSync(c.Context(), id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(out)
}
