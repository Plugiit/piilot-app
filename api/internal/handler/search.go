package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// SearchService est ce que le handler attend du usecase.
type SearchService interface {
	Search(ctx context.Context, query string, scope usecase.SearchScope) (usecase.SearchResult, error)
}

// Search sert la palette de recherche globale.
type Search struct {
	svc SearchService
}

func NewSearch(svc SearchService) *Search { return &Search{svc: svc} }

// Get cherche dans tout ce que l'appelant a le droit de voir. Le perimetre
// suit ses permissions, famille par famille : un compte sans tickets.read
// cherche quand meme ses projets.
func (h *Search) Get(c fiber.Ctx) error {
	var scope usecase.SearchScope
	for _, p := range []struct {
		permission string
		flag       *bool
	}{
		{"projects.read", &scope.Projects},
		{"clients.read", &scope.Clients},
		{"tasks.read", &scope.Tasks},
		{"tickets.read", &scope.Tickets},
	} {
		ok, err := middleware.Can(c, p.permission)
		if err != nil {
			return err
		}
		*p.flag = ok
	}

	result, err := h.svc.Search(c.Context(), c.Query("q"), scope)
	if err != nil {
		return err
	}

	return c.JSON(result)
}
