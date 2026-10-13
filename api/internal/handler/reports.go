package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ReportService est ce que le handler attend du rapport mensuel.
type ReportService interface {
	Settings(ctx context.Context) (usecase.ReportSettings, error)
	UpdateSettings(ctx context.Context, in usecase.ReportSettings) (usecase.ReportSettings, error)
	Generate(ctx context.Context, month time.Time, projectID *uuid.UUID) (int, error)
	List(ctx context.Context, month time.Time, status *string, page, pageSize int) (usecase.ReportPage, error)
	Get(ctx context.Context, id uuid.UUID) (usecase.ReportView, error)
	SetEditorial(ctx context.Context, id uuid.UUID, text string) (usecase.ReportView, error)
	Regenerate(ctx context.Context, id uuid.UUID) (usecase.ReportView, error)
	Validate(ctx context.Context, id, userID uuid.UUID) (usecase.ReportView, error)
	Unvalidate(ctx context.Context, id uuid.UUID) (usecase.ReportView, error)
	Send(ctx context.Context, id uuid.UUID) (usecase.ReportView, error)
	Recipients(ctx context.Context, projectID uuid.UUID) (usecase.ReportRecipientsView, error)
	SetRecipients(ctx context.Context, projectID uuid.UUID, excluded []uuid.UUID, extra []string) (usecase.ReportRecipientsView, error)
	PortalReports(ctx context.Context, userID, projectID uuid.UUID) ([]usecase.PortalReportItem, error)
	PortalReport(ctx context.Context, userID, id uuid.UUID) (usecase.PortalReport, error)
}

// Reports porte le rapport mensuel de maintenance.
type Reports struct {
	svc ReportService
}

// NewReports construit le handler.
func NewReports(svc ReportService) *Reports { return &Reports{svc: svc} }

func parseMonth(raw string) (time.Time, error) {
	if raw == "" {
		return usecase.CurrentMonth().AddDate(0, -1, 0), nil
	}
	m, err := time.Parse("2006-01", raw)
	if err != nil {
		return time.Time{}, domain.ErrValidation.WithDetails(map[string]any{"month": "Un mois au format AAAA-MM"})
	}
	return m, nil
}

func reportID(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	return id, nil
}

// Settings rend les reglages d'envoi.
func (h *Reports) Settings(c fiber.Ctx) error {
	out, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// UpdateSettings enregistre les reglages d'envoi.
func (h *Reports) UpdateSettings(c fiber.Ctx) error {
	var b usecase.ReportSettings
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.UpdateSettings(c.Context(), b)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Generate cree les brouillons manquants d'un mois.
func (h *Reports) Generate(c fiber.Ctx) error {
	var b struct {
		Month     string  `json:"month"`
		ProjectID *string `json:"project_id"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	month, err := parseMonth(b.Month)
	if err != nil {
		return err
	}
	var project *uuid.UUID
	if b.ProjectID != nil && *b.ProjectID != "" {
		p, err := uuid.Parse(*b.ProjectID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"project_id": "Identifiant invalide"})
		}
		project = &p
	}
	n, err := h.svc.Generate(c.Context(), month, project)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"created": n})
}

// List rend les rapports d'un mois. ?month=AAAA-MM (le mois ecoule par
// defaut), ?status=draft|validated|sent.
func (h *Reports) List(c fiber.Ctx) error {
	month, err := parseMonth(c.Query("month"))
	if err != nil {
		return err
	}
	var status *string
	if raw := c.Query("status"); raw != "" {
		if raw != "draft" && raw != "validated" && raw != "sent" {
			return domain.ErrValidation.WithDetails(map[string]any{"status": "draft, validated ou sent"})
		}
		status = &raw
	}
	out, err := h.svc.List(c.Context(), month, status, queryInt(c, "page", 1), queryInt(c, "page_size", 25))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Get rend un rapport.
func (h *Reports) Get(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Editorial enregistre le mot du mois.
func (h *Reports) Editorial(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var b struct {
		Editorial string `json:"editorial"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.SetEditorial(c.Context(), id, b.Editorial)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (h *Reports) action(c fiber.Ctx, run func(context.Context, uuid.UUID) (usecase.ReportView, error)) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	out, err := run(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Regenerate refait les donnees d'un brouillon.
func (h *Reports) Regenerate(c fiber.Ctx) error { return h.action(c, h.svc.Regenerate) }

// Validate fige un brouillon.
func (h *Reports) Validate(c fiber.Ctx) error {
	user, err := caller(c)
	if err != nil {
		return err
	}
	return h.action(c, func(ctx context.Context, id uuid.UUID) (usecase.ReportView, error) {
		return h.svc.Validate(ctx, id, user)
	})
}

// Unvalidate repasse un rapport valide en brouillon.
func (h *Reports) Unvalidate(c fiber.Ctx) error { return h.action(c, h.svc.Unvalidate) }

// Send envoie un rapport valide.
func (h *Reports) Send(c fiber.Ctx) error { return h.action(c, h.svc.Send) }

// Recipients rend les destinataires des rapports d'un projet.
func (h *Reports) Recipients(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Recipients(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// SetRecipients remplace les exclusions et les adresses d'un projet.
func (h *Reports) SetRecipients(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var b struct {
		Excluded []string `json:"excluded"`
		Extra    []string `json:"extra"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	excluded, err := parseUUIDs(b.Excluded, "excluded")
	if err != nil {
		return err
	}
	out, err := h.svc.SetRecipients(c.Context(), id, excluded, b.Extra)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// PortalReports rend l'archive des rapports d'un projet du client.
func (h *Reports) PortalReports(c fiber.Ctx) error {
	user, id, err := portalProject(c)
	if err != nil {
		return err
	}
	out, err := h.svc.PortalReports(c.Context(), user, id)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": out})
}

// PortalReport rend un rapport envoye, pour le client du projet.
func (h *Reports) PortalReport(c fiber.Ctx) error {
	user, id, err := portalProject(c)
	if err != nil {
		return err
	}
	out, err := h.svc.PortalReport(c.Context(), user, id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}
