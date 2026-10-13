package handler

import (
	"context"
	"regexp"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// AnalyticsService est ce que le handler attend du module de statistiques.
type AnalyticsService interface {
	Settings(ctx context.Context) (usecase.AnalyticsSettingsView, error)
	UpdateSettings(ctx context.Context, in usecase.AnalyticsSettingsInput) (usecase.AnalyticsSettingsView, error)
	RequestCheck(ctx context.Context) (usecase.AnalyticsSettingsView, error)
	Websites(ctx context.Context, query string, page, pageSize int) (usecase.AnalyticsWebsitePage, error)
	ProjectAnalytics(ctx context.Context, projectID uuid.UUID) (usecase.ProjectAnalyticsView, error)
	SetProjectWebsites(ctx context.Context, projectID uuid.UUID, sites []usecase.ProjectWebsiteInput) (usecase.ProjectAnalyticsView, error)
	PortalStatistics(ctx context.Context, userID, projectID uuid.UUID, key string) (usecase.StatisticsView, error)
	PortalActive(ctx context.Context, userID, projectID uuid.UUID) (usecase.ActiveView, error)
}

// Analytics porte le module de statistiques : reglages, sites, liens aux
// projets, et la page « Statistiques » du portail.
type Analytics struct {
	svc AnalyticsService
}

// NewAnalytics construit le handler.
func NewAnalytics(svc AnalyticsService) *Analytics { return &Analytics{svc: svc} }

// Settings rend les reglages du module.
func (h *Analytics) Settings(c fiber.Ctx) error {
	out, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type analyticsSettingsRequest struct {
	Enabled      bool    `json:"enabled"`
	BaseURL      string  `json:"base_url"`
	AuthMode     string  `json:"auth_mode"`
	Username     string  `json:"username"`
	Password     *string `json:"password"`
	APIKey       *string `json:"api_key"`
	APIVersion   int     `json:"api_version"`
	CacheMinutes int     `json:"cache_minutes"`
}

// UpdateSettings enregistre les reglages du module.
func (h *Analytics) UpdateSettings(c fiber.Ctx) error {
	var b analyticsSettingsRequest
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.UpdateSettings(c.Context(), usecase.AnalyticsSettingsInput{
		Enabled: b.Enabled, BaseURL: b.BaseURL, AuthMode: b.AuthMode, Username: b.Username,
		Password: b.Password, APIKey: b.APIKey, APIVersion: b.APIVersion, CacheMinutes: b.CacheMinutes,
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Check demande une verification : le « Tester la connexion » de l'ecran.
func (h *Analytics) Check(c fiber.Ctx) error {
	out, err := h.svc.RequestCheck(c.Context())
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(out)
}

// Websites rend une page des sites du service.
func (h *Analytics) Websites(c fiber.Ctx) error {
	out, err := h.svc.Websites(c.Context(), c.Query("q"), queryInt(c, "page", 1), queryInt(c, "page_size", 25))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Project rend les sites lies a un projet.
func (h *Analytics) Project(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	out, err := h.svc.ProjectAnalytics(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type projectWebsitesRequest struct {
	Sites []struct {
		WebsiteID   string `json:"website_id"`
		Environment string `json:"environment"`
	} `json:"sites"`
}

// SetProject remplace les sites d'un projet.
func (h *Analytics) SetProject(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	var b projectWebsitesRequest
	if err := c.Bind().Body(&b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	sites := make([]usecase.ProjectWebsiteInput, 0, len(b.Sites))
	for _, s := range b.Sites {
		website, err := uuid.Parse(s.WebsiteID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"sites": "Identifiant invalide"})
		}
		sites = append(sites, usecase.ProjectWebsiteInput{WebsiteID: website, Environment: s.Environment})
	}
	out, err := h.svc.SetProjectWebsites(c.Context(), id, sites)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Statistics rend la page « Statistiques » d'un projet du client.
// ?period=7d|30d|month|prev_month|12m, ou ?period=custom&from=…&to=….
func (h *Analytics) Statistics(c fiber.Ctx) error {
	user, id, err := portalProject(c)
	if err != nil {
		return err
	}
	key := c.Query("period", "30d")
	if key == "custom" {
		from, to := c.Query("from"), c.Query("to")
		if !isoDate.MatchString(from) || !isoDate.MatchString(to) {
			return domain.ErrValidation.WithDetails(map[string]any{"period": "from et to attendus, au format AAAA-MM-JJ"})
		}
		key = "custom:" + from + ":" + to
	}
	out, err := h.svc.PortalStatistics(c.Context(), user, id, key)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Active rend les visiteurs du moment, relus toutes les trente secondes par
// la page.
func (h *Analytics) Active(c fiber.Ctx) error {
	user, id, err := portalProject(c)
	if err != nil {
		return err
	}
	out, err := h.svc.PortalActive(c.Context(), user, id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// portalProject lit l'appelant et le projet de l'adresse.
func portalProject(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	user, err := caller(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}
	return user, id, nil
}
