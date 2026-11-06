package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// DiscordService est ce que les routes de la liaison attendent : celles du
// compte connecte et celles du bot.
type DiscordService interface {
	Status(ctx context.Context, userID uuid.UUID) (usecase.DiscordStatus, error)
	CreateCode(ctx context.Context, userID uuid.UUID) (usecase.DiscordCode, error)
	Unlink(ctx context.Context, userID uuid.UUID) error
	Redeem(ctx context.Context, code, discordUserID string) (usecase.DiscordLinkCreated, error)
	Link(ctx context.Context, discordUserID string) (usecase.DiscordLink, error)
	Links(ctx context.Context, after string, limit int) (usecase.DiscordLinkPage, error)
	RateKey(discordUserID string) string
}

// Bornes du rachat d'un code par le bot.
//
// Par membre : un humain qui se trompe de code y tient largement. Au total :
// c'est la borne qui compte, car un identifiant Discord se cree a volonte. 60
// essais par quart d'heure sur 2^40 codes, dont quelques-uns actifs a la fois,
// laissent une chance de tomber juste qui se compte en milliardiemes par jour.
const (
	redeemMaxPerMember = 5
	redeemMaxGlobal    = 60
	redeemWindow       = 15 * time.Minute
)

// errRedeemUnavailable : sans compteur, le rachat est refuse. A l'inverse de
// la connexion, l'echec est ferme : un /lier remis a plus tard coute peu, une
// limite levee sur des codes de 40 bits coute plus.
var errRedeemUnavailable = &domain.Error{
	Status: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE",
	Message: "Service momentanément indisponible, réessayez dans quelques minutes",
}

// Discord porte la liaison d'un compte Discord.
type Discord struct {
	svc     DiscordService
	limiter RateLimiter
}

// NewDiscord construit le handler.
func NewDiscord(svc DiscordService, limiter RateLimiter) *Discord {
	return &Discord{svc: svc, limiter: limiter}
}

// Status rend la carte « Discord » du compte connecte.
func (h *Discord) Status(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	out, err := h.svc.Status(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// CreateCode tire un code de liaison et invalide le precedent.
func (h *Discord) CreateCode(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	out, err := h.svc.CreateCode(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

// Unlink supprime la liaison du compte connecte.
func (h *Discord) Unlink(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	if err := h.svc.Unlink(c.Context(), userID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Redeem rachete un code pour le bot : POST /bot/discord-links.
func (h *Discord) Redeem(c fiber.Ctx) error {
	// Des champs lus tels quels : un identifiant Discord envoye en nombre JSON
	// a deja perdu ses derniers chiffres, il est refuse plutot que converti.
	var b struct {
		Code          any `json:"code"`
		DiscordUserID any `json:"discord_user_id"`
	}
	if err := json.Unmarshal(c.Body(), &b); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	code, _ := b.Code.(string)
	discordUserID, _ := b.DiscordUserID.(string)
	if err := usecase.ValidateRedeem(code, discordUserID); err != nil {
		return err
	}

	// Pas de remise a zero apres un rachat reussi, a l'inverse de la
	// connexion : un membre qui tient un compte pourrait sinon enchainer
	// quatre essais, un rachat de son propre code, et recommencer.
	if err := h.checkRedeemRate(c, "discord-redeem:member:"+h.svc.RateKey(discordUserID)); err != nil {
		return err
	}

	out, err := h.svc.Redeem(c.Context(), code, discordUserID)
	if err != nil {
		return err
	}
	// Le journal prete la liaison au compte qui a tire le code : c'est lui qui
	// a prouve tenir les deux comptes.
	c.Locals(auditActorKey, out.UserID)

	return c.Status(fiber.StatusCreated).JSON(out)
}

// Link rend la liaison d'un membre : GET /bot/discord-links/{discord_user_id}.
func (h *Discord) Link(c fiber.Ctx) error {
	out, err := h.svc.Link(c.Context(), c.Params("discord_user_id"))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Links rend une page de toutes les liaisons : GET /bot/discord-links.
func (h *Discord) Links(c fiber.Ctx) error {
	limit, err := botLimit(c)
	if err != nil {
		return err
	}
	after := c.Query("after")
	if after != "" && !usecase.ValidDiscordUserID(after) {
		return domain.ErrValidation.WithDetails(map[string]any{"after": "Le next_after de la page précédente"})
	}
	out, err := h.svc.Links(c.Context(), after, limit)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// checkRedeemRate refuse le rachat si un compteur deborde. Le membre d'abord :
// un membre qui insiste n'entame pas la borne commune.
func (h *Discord) checkRedeemRate(c fiber.Ctx, member string) error {
	for _, k := range []struct {
		key   string
		limit int
	}{
		{key: member, limit: redeemMaxPerMember},
		{key: "discord-redeem:global", limit: redeemMaxGlobal},
	} {
		allowed, retryAfter, err := h.limiter.Allow(c.Context(), k.key, k.limit, redeemWindow)
		if err != nil {
			return errRedeemUnavailable.WithCause(err)
		}
		if !allowed {
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(max(1, int(retryAfter.Seconds()))))
			return domain.ErrRateLimited
		}
	}
	return nil
}

// botLimit lit la taille de page des routes du bot : 1 a 100, 100 par defaut.
func botLimit(c fiber.Ctx) (int, error) {
	raw := c.Query("limit")
	if raw == "" {
		return usecase.BotPageMax, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > usecase.BotPageMax {
		return 0, domain.ErrValidation.WithDetails(map[string]any{
			"limit": "Un entier de 1 à " + strconv.Itoa(usecase.BotPageMax),
		})
	}
	return limit, nil
}
