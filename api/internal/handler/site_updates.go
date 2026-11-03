package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// SiteUpdateService est ce que le handler attend des mises a jour des sites.
type SiteUpdateService interface {
	View(ctx context.Context, projectID uuid.UUID) (usecase.SiteUpdatesView, error)
	SetTarget(ctx context.Context, projectID uuid.UUID, in usecase.SiteUpdateTargetInput) (usecase.SiteUpdatesView, error)
	RequestRun(ctx context.Context, projectID, userID uuid.UUID, confirmedEnvironment string) (usecase.SiteUpdatesView, error)
	RecordResult(ctx context.Context, token string, in usecase.SiteUpdateReport) error
	TokenStatus(ctx context.Context) (usecase.SiteUpdateTokenStatus, error)
	SetToken(ctx context.Context, token string) (usecase.SiteUpdateTokenStatus, error)
	ClearToken(ctx context.Context) (usecase.SiteUpdateTokenStatus, error)
}

// SiteUpdates porte les mises a jour des sites.
type SiteUpdates struct {
	svc SiteUpdateService
}

// NewSiteUpdates construit le handler.
func NewSiteUpdates(svc SiteUpdateService) *SiteUpdates { return &SiteUpdates{svc: svc} }

// View rend la carte d'un projet.
func (h *SiteUpdates) View(c fiber.Ctx) error {
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

// SetTarget enregistre l'environnement choisi pour un projet.
func (h *SiteUpdates) SetTarget(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var b struct {
		Environment string `json:"environment"`
		Workflow    string `json:"workflow"`
		Ref         string `json:"ref"`
		EveryDays   *int   `json:"every_days"`
		Enabled     bool   `json:"enabled"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.SetTarget(c.Context(), id, usecase.SiteUpdateTargetInput{
		Environment: b.Environment, Workflow: b.Workflow, Ref: b.Ref, EveryDays: b.EveryDays, Enabled: b.Enabled,
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// RequestRun demande un lancement.
func (h *SiteUpdates) RequestRun(c fiber.Ctx) error {
	id, err := reportID(c)
	if err != nil {
		return err
	}
	user, err := caller(c)
	if err != nil {
		return err
	}
	var b struct {
		Environment string `json:"environment"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.RequestRun(c.Context(), id, user, b.Environment)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Hook recoit le compte rendu de la CI. Sans session : le jeton du lancement,
// valable pour lui seul, fait office de preuve.
func (h *SiteUpdates) Hook(c fiber.Ctx) error {
	token, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	if !ok {
		return usecase.ErrSiteUpdateForbidden
	}
	var b struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		RunURL  string `json:"run_url"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	if err := h.svc.RecordResult(c.Context(), token, usecase.SiteUpdateReport{Status: b.Status, Message: b.Message, RunURL: b.RunURL}); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// TokenStatus dit si le jeton GitHub est enregistre.
func (h *SiteUpdates) TokenStatus(c fiber.Ctx) error {
	out, err := h.svc.TokenStatus(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// SetToken enregistre le jeton GitHub.
func (h *SiteUpdates) SetToken(c fiber.Ctx) error {
	var b struct {
		Token string `json:"token"`
	}
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.SetToken(c.Context(), b.Token)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// ClearToken retire le jeton GitHub.
func (h *SiteUpdates) ClearToken(c fiber.Ctx) error {
	out, err := h.svc.ClearToken(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}
