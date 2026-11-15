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

// RgpdErasureService est le contrat dont l'ecran des effacements a besoin.
type RgpdErasureService interface {
	Request(ctx context.Context, actor *uuid.UUID, subjectType string, subjectID uuid.UUID) (usecase.RgpdErasureItem, error)
	Cancel(ctx context.Context, actor *uuid.UUID, id uuid.UUID) (usecase.RgpdErasureItem, error)
	List(ctx context.Context, filter usecase.RgpdErasureFilter, page int) (usecase.RgpdErasurePage, error)
	Settings() usecase.RgpdSettings
}

// Rgpd porte les endpoints RGPD.
type Rgpd struct {
	exports  RgpdExportService
	erasures RgpdErasureService
}

// NewRgpd construit le handler.
func NewRgpd(exports RgpdExportService, erasures RgpdErasureService) *Rgpd {
	return &Rgpd{exports: exports, erasures: erasures}
}

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

// CreateErasure planifie un effacement : 201, l'execution a lieu a l'echeance.
func (h *Rgpd) CreateErasure(c fiber.Ctx) error {
	var req requestRgpdExport
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	id, err := uuid.Parse(strings.TrimSpace(req.SubjectID))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"subject_id": "Identifiant invalide"})
	}

	item, err := h.erasures.Request(c.Context(), actorOf(c), req.SubjectType, id)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

// ListErasures sert l'historique des effacements, filtrable par statut et par
// sujet (subject_type, subject_id). Une valeur invalide : 422.
func (h *Rgpd) ListErasures(c fiber.Ctx) error {
	filter := usecase.RgpdErasureFilter{
		Status:      strings.TrimSpace(c.Query("status")),
		SubjectType: strings.TrimSpace(c.Query("subject_type")),
	}

	switch filter.SubjectType {
	case "", usecase.RgpdSubjectClient, usecase.RgpdSubjectUser:
	default:
		return domain.ErrValidation.WithDetails(map[string]any{"subject_type": "Type inconnu : client ou user"})
	}

	if raw := strings.TrimSpace(c.Query("subject_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"subject_id": "Identifiant invalide"})
		}

		filter.SubjectID = &id
	}

	page, err := h.erasures.List(c.Context(), filter, queryInt(c, "page", 1))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// CancelErasure annule un effacement planifie.
func (h *Rgpd) CancelErasure(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	item, err := h.erasures.Cancel(c.Context(), actorOf(c), id)
	if err != nil {
		return err
	}

	return c.JSON(item)
}

// Settings sert les delais RGPD en vigueur.
func (h *Rgpd) Settings(c fiber.Ctx) error {
	return c.JSON(h.erasures.Settings())
}
