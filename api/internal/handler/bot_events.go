package handler

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// BotEventService est ce que le flux du bot attend.
type BotEventService interface {
	Events(ctx context.Context, after *int64, limit int) (usecase.BotEventPage, error)
}

// BotEvents sert le flux d'evenements que le bot relit.
type BotEvents struct {
	svc BotEventService
}

// NewBotEvents construit le handler.
func NewBotEvents(svc BotEventService) *BotEvents { return &BotEvents{svc: svc} }

// List rend les evenements qui suivent un curseur : GET /bot/events.
//
// Pagination par curseur et non par page : un flux n'a pas de total, et une
// page numerotee glisserait a chaque evenement ecrit pendant la lecture.
func (h *BotEvents) List(c fiber.Ctx) error {
	limit, err := botLimit(c)
	if err != nil {
		return err
	}
	var after *int64
	if raw := c.Query("after"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return domain.ErrValidation.WithDetails(map[string]any{"after": "Le next_after de la page précédente"})
		}
		after = &n
	}
	out, err := h.svc.Events(c.Context(), after, limit)
	if err != nil {
		return err
	}
	return c.JSON(out)
}
