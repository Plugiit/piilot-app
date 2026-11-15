package handler

import (
	"context"
	"io"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// RgpdExportService est le contrat dont l'ecran RGPD a besoin.
type RgpdExportService interface {
	Request(ctx context.Context, actor *uuid.UUID, subjectType string, subjectID uuid.UUID) (usecase.RgpdExportItem, error)
	List(ctx context.Context, page int) (usecase.RgpdExportPage, error)
	Open(ctx context.Context, id uuid.UUID) (usecase.RgpdExportItem, io.ReadCloser, error)
}

// Rgpd porte les endpoints RGPD.
type Rgpd struct {
	exports RgpdExportService
}

// NewRgpd construit le handler.
func NewRgpd(exports RgpdExportService) *Rgpd { return &Rgpd{exports: exports} }

// requestRgpdExport est le corps de POST /admin/rgpd/exports.
type requestRgpdExport struct {
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
}

// CreateExport met un export en file : 202, la construction suit en tache de fond.
func (h *Rgpd) CreateExport(c fiber.Ctx) error {
	var req requestRgpdExport
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	id, err := uuid.Parse(strings.TrimSpace(req.SubjectID))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"subject_id": "Identifiant invalide"})
	}

	item, err := h.exports.Request(c.Context(), actorOf(c), req.SubjectType, id)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(item)
}

// ListExports sert l'historique des exports.
func (h *Rgpd) ListExports(c fiber.Ctx) error {
	page, err := h.exports.List(c.Context(), queryInt(c, "page", 1))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// DownloadExport sert l'archive d'un export pret.
func (h *Rgpd) DownloadExport(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	item, content, err := h.exports.Open(c.Context(), id)
	if err != nil {
		return err
	}

	// Pas de `defer Close` : le corps est ecrit apres le retour du handler,
	// c'est Fiber qui referme le lecteur.
	name := "piilot-rgpd-" + item.SubjectType + "-" + item.ID.String()[:8] + ".zip"

	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderContentDisposition, contentDisposition(name))
	c.Set(fiber.HeaderCacheControl, "no-store")

	return c.SendStream(content, int(item.SizeBytes))
}
