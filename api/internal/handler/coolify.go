package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// CoolifyService est ce que le handler attend du module de deploiements.
type CoolifyService interface {
	Settings(ctx context.Context) (usecase.CoolifySettingsView, error)
	UpdateSettings(ctx context.Context, in usecase.CoolifySettingsInput) (usecase.CoolifySettingsView, error)
	RequestPoll(ctx context.Context) (usecase.CoolifySettingsView, error)
	Applications(ctx context.Context) ([]usecase.CoolifyApplicationView, error)
}

// Coolify porte le module de deploiements.
type Coolify struct {
	svc CoolifyService
}

// NewCoolify construit le handler.
func NewCoolify(svc CoolifyService) *Coolify { return &Coolify{svc: svc} }

// Settings rend les reglages du module et l'etat de la releve.
func (h *Coolify) Settings(c fiber.Ctx) error {
	out, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type coolifySettingsRequest struct {
	Enabled     bool    `json:"enabled"`
	BaseURL     string  `json:"base_url"`
	APIToken    *string `json:"api_token"`
	PollMinutes int     `json:"poll_minutes"`
}

// UpdateSettings enregistre les reglages du module.
func (h *Coolify) UpdateSettings(c fiber.Ctx) error {
	var body coolifySettingsRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.UpdateSettings(c.Context(), usecase.CoolifySettingsInput{
		Enabled: body.Enabled, BaseURL: body.BaseURL, APIToken: body.APIToken, PollMinutes: body.PollMinutes,
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Poll demande une releve : le « Tester la connexion » de l'ecran.
func (h *Coolify) Poll(c fiber.Ctx) error {
	out, err := h.svc.RequestPoll(c.Context())
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(out)
}

// Applications rend les applications relevees.
func (h *Coolify) Applications(c fiber.Ctx) error {
	out, err := h.svc.Applications(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": out})
}
