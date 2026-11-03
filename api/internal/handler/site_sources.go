package handler

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// SiteSourcesService est ce que le handler attend des sauvegardes et domaines.
type SiteSourcesService interface {
	View(ctx context.Context, projectID uuid.UUID) (usecase.SiteSourcesView, error)
	RotateBackupToken(ctx context.Context, projectID uuid.UUID) (usecase.SiteBackupToken, error)
	UpdateBackup(ctx context.Context, projectID uuid.UUID, expectedHours int, restoreTestOn *time.Time) (usecase.SiteSourcesView, error)
	RemoveBackup(ctx context.Context, projectID uuid.UUID) (usecase.SiteSourcesView, error)
	RecordBackup(ctx context.Context, token string, in usecase.SiteBackupReport) error
	AddDomain(ctx context.Context, projectID uuid.UUID, raw string) (usecase.SiteSourcesView, error)
	RemoveDomain(ctx context.Context, projectID, id uuid.UUID) (usecase.SiteSourcesView, error)
}

// SiteSources porte les sauvegardes et les domaines des sites.
type SiteSources struct {
	svc SiteSourcesService
}

// NewSiteSources construit le handler.
func NewSiteSources(svc SiteSourcesService) *SiteSources { return &SiteSources{svc: svc} }

// View rend la carte d'un projet.
func (h *SiteSources) View(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.View(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// RotateToken tire un jeton, montre une seule fois.
func (h *SiteSources) RotateToken(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.RotateBackupToken(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// UpdateBackup enregistre le delai attendu et le dernier test de restauration.
func (h *SiteSources) UpdateBackup(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var b struct {
		ExpectedHours     int     `json:"expected_hours"`
		LastRestoreTestOn *string `json:"last_restore_test_on"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	var restore *time.Time
	if b.LastRestoreTestOn != nil && *b.LastRestoreTestOn != "" {
		day, err := time.Parse("2006-01-02", *b.LastRestoreTestOn)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"last_restore_test_on": "Une date au format AAAA-MM-JJ"})
		}
		restore = &day
	}
	out, err := h.svc.UpdateBackup(c.Context(), id, b.ExpectedHours, restore)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// RemoveBackup cesse de surveiller les sauvegardes d'un projet.
func (h *SiteSources) RemoveBackup(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.RemoveBackup(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// BackupHook recoit le signal d'une sauvegarde. Sans session : le jeton du
// projet, en en-tete pour ne pas finir dans les journaux d'acces, fait preuve.
func (h *SiteSources) BackupHook(c fiber.Ctx) error {
	token, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	if !ok {
		return usecase.ErrSiteBackupForbidden
	}
	var b struct {
		Status     string     `json:"status"`
		SizeBytes  *int64     `json:"size_bytes"`
		Message    string     `json:"message"`
		FinishedAt *time.Time `json:"finished_at"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	if err := h.svc.RecordBackup(c.Context(), token, usecase.SiteBackupReport{
		Status: b.Status, SizeBytes: b.SizeBytes, Message: b.Message, FinishedAt: b.FinishedAt,
	}); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AddDomain suit un domaine de plus.
func (h *SiteSources) AddDomain(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var b struct {
		Domain string `json:"domain"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.AddDomain(c.Context(), id, b.Domain)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// RemoveDomain cesse de suivre un domaine.
func (h *SiteSources) RemoveDomain(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	domainID, err := uuid.Parse(c.Params("domainId"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"domainId": "Identifiant invalide"})
	}
	out, err := h.svc.RemoveDomain(c.Context(), id, domainID)
	if err != nil {
		return err
	}
	return c.JSON(out)
}
