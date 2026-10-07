package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// BackupService est le contrat de l'ecran des sauvegardes.
type BackupService interface {
	Status(ctx context.Context) (usecase.BackupStatus, error)
}

// Backups sert l'etat des sauvegardes aux admins.
type Backups struct {
	svc BackupService
}

// NewBackups construit le handler.
func NewBackups(svc BackupService) *Backups {
	return &Backups{svc: svc}
}

// Status sert la derniere sauvegarde reussie et le journal.
func (h *Backups) Status(c fiber.Ctx) error {
	status, err := h.svc.Status(c.Context())
	if err != nil {
		return err
	}

	return c.JSON(status)
}
