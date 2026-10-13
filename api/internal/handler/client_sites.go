package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ClientSites sert le bandeau des sites du portail : l'audience (Umami) et la
// disponibilite (Uptime Kuma) des sites de production du client, chacune lue
// dans sa table locale.
type ClientSites struct {
	audience interface {
		PortalAudience(ctx context.Context, userID uuid.UUID) (usecase.AudienceView, error)
	}
	uptime interface {
		PortalUptime(ctx context.Context, userID uuid.UUID) ([]usecase.PortalUptimeSite, error)
	}
}

// NewClientSites construit le handler.
func NewClientSites(analytics *usecase.AnalyticsService, uptime *usecase.UptimeService) *ClientSites {
	return &ClientSites{audience: analytics, uptime: uptime}
}

// clientSitesView est la reponse : les deux listes, vides quand le module
// correspondant est coupe.
type clientSitesView struct {
	Audience []usecase.AudienceSite     `json:"audience"`
	Uptime   []usecase.PortalUptimeSite `json:"uptime"`
}

// Get rend les sites du client.
func (h *ClientSites) Get(c fiber.Ctx) error {
	user, err := caller(c)
	if err != nil {
		return err
	}
	audience, err := h.audience.PortalAudience(c.Context(), user)
	if err != nil {
		return err
	}
	uptime, err := h.uptime.PortalUptime(c.Context(), user)
	if err != nil {
		return err
	}
	return c.JSON(clientSitesView{Audience: audience.Sites, Uptime: uptime})
}
