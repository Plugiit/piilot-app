package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// UptimeService est ce que le handler attend du module de disponibilite.
type UptimeService interface {
	Settings(ctx context.Context) (usecase.UptimeSettingsView, error)
	UpdateSettings(ctx context.Context, in usecase.UptimeSettingsInput) (usecase.UptimeSettingsView, error)
	RotateWebhookSecret(ctx context.Context) (usecase.UptimeSettingsView, error)
	RequestPoll(ctx context.Context) (usecase.UptimeSettingsView, error)
	Webhook(ctx context.Context, provider, token string, body []byte) (usecase.UptimeWebhookResult, error)
	Monitors(ctx context.Context, query string, page, pageSize int) (usecase.UptimeMonitorPage, error)
	Incidents(ctx context.Context, page, pageSize int) (usecase.UptimeIncidentPage, error)
	ProjectUptime(ctx context.Context, projectID uuid.UUID) (usecase.ProjectUptimeView, error)
	SetProjectSites(ctx context.Context, projectID uuid.UUID, sites []usecase.ProjectSiteInput) (usecase.ProjectUptimeView, error)
}

// Uptime porte les modules et la disponibilite des sites.
type Uptime struct {
	svc UptimeService
}

// NewUptime construit le handler.
func NewUptime(svc UptimeService) *Uptime { return &Uptime{svc: svc} }

// Settings rend les reglages du module et l'etat de la releve.
func (h *Uptime) Settings(c fiber.Ctx) error {
	out, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type uptimeSettingsRequest struct {
	Enabled     bool    `json:"enabled"`
	BaseURL     string  `json:"base_url"`
	APIKey      *string `json:"api_key"`
	PollMinutes int     `json:"poll_minutes"`
}

// UpdateSettings enregistre les reglages du module.
func (h *Uptime) UpdateSettings(c fiber.Ctx) error {
	var body uptimeSettingsRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.UpdateSettings(c.Context(), usecase.UptimeSettingsInput{
		Enabled: body.Enabled, BaseURL: body.BaseURL, APIKey: body.APIKey, PollMinutes: body.PollMinutes,
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Rotate tire un nouveau secret de webhook.
func (h *Uptime) Rotate(c fiber.Ctx) error {
	out, err := h.svc.RotateWebhookSecret(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Poll demande une releve : le « Tester la connexion » de l'ecran.
func (h *Uptime) Poll(c fiber.Ctx) error {
	out, err := h.svc.RequestPoll(c.Context())
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(out)
}

// Webhook recoit un changement d'etat pousse par le service. Sans session :
// le jeton de l'adresse fait office de preuve.
func (h *Uptime) Webhook(c fiber.Ctx) error {
	out, err := h.svc.Webhook(c.Context(), c.Params("provider"), c.Query("token"), c.Body())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Monitors rend une page de sites.
func (h *Uptime) Monitors(c fiber.Ctx) error {
	out, err := h.svc.Monitors(c.Context(), c.Query("q"), queryInt(c, "page", 1), queryInt(c, "page_size", 25))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Incidents rend une page d'incidents.
func (h *Uptime) Incidents(c fiber.Ctx) error {
	out, err := h.svc.Incidents(c.Context(), queryInt(c, "page", 1), queryInt(c, "page_size", 10))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Project rend l'onglet « Disponibilite » d'un projet.
func (h *Uptime) Project(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	out, err := h.svc.ProjectUptime(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type projectSitesRequest struct {
	Sites []struct {
		MonitorID   string `json:"monitor_id"`
		Environment string `json:"environment"`
	} `json:"sites"`
}

// SetProject remplace les sites d'un projet.
func (h *Uptime) SetProject(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	var body projectSitesRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	sites := make([]usecase.ProjectSiteInput, 0, len(body.Sites))
	for _, s := range body.Sites {
		monitor, err := uuid.Parse(s.MonitorID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"sites": "Identifiant invalide"})
		}
		sites = append(sites, usecase.ProjectSiteInput{MonitorID: monitor, Environment: s.Environment})
	}
	out, err := h.svc.SetProjectSites(c.Context(), id, sites)
	if err != nil {
		return err
	}
	return c.JSON(out)
}
