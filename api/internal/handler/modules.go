package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ModuleSource est un module qui se presente sur la page des modules.
type ModuleSource interface {
	Modules(ctx context.Context) ([]usecase.ModuleView, error)
}

// Modules rassemble les cartes de la page des modules.
type Modules struct {
	sources []ModuleSource
}

// NewModules construit le handler.
func NewModules(sources ...ModuleSource) *Modules { return &Modules{sources: sources} }

// List rend les modules activables.
func (h *Modules) List(c fiber.Ctx) error {
	out := []usecase.ModuleView{}
	for _, src := range h.sources {
		items, err := src.Modules(c.Context())
		if err != nil {
			return err
		}
		out = append(out, items...)
	}
	return c.JSON(fiber.Map{"items": out})
}
