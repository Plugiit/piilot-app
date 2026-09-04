package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/middleware"
)

// Deps regroupe les handlers a monter sur le routeur.
type Deps struct {
	Health *Health
	Auth   *Auth
	Guard  *middleware.Guard
}

// Register monte toutes les routes de l'API.
//
// Les endpoints sont organises par vue, pas par entite : chaque route renvoie
// exactement ce qu'un ecran affiche. C'est le choix qui evite le sur-fetch et
// garde 1 a 3 requetes SQL par appel.
func Register(app *fiber.App, deps Deps) {
	// Sondes, hors versionnement : leur contrat ne bouge pas.
	app.Get("/health/live", deps.Health.Live)
	app.Get("/health/ready", deps.Health.Ready)

	// Contrat HTTP, hors versionnement lui aussi : c'est lui qui porte les
	// versions des routes. Le front le consomme pour generer ses types.
	app.Get("/openapi.json", openapiSpec)

	v1 := app.Group("/api/v1")

	registerAuthRoutes(v1.Group("/auth"), deps)
	registerAdminRoutes(v1.Group("/admin"), deps)
}

// registerAuthRoutes monte les endpoints publics d'authentification.
//
// login, logout et refresh sont ouverts : ils portent leur propre preuve
// (identifiants ou jeton de rafraichissement) et ne peuvent donc pas exiger
// une session deja etablie.
func registerAuthRoutes(r fiber.Router, deps Deps) {
	r.Post("/login", deps.Auth.Login)
	r.Post("/logout", deps.Auth.Logout)
	r.Post("/refresh", deps.Auth.Refresh)

	// La garde vient AVANT le handler : Fiber execute les handlers dans l'ordre
	// ou ils sont passes. Une garde placee apres ne s'executerait que si le
	// handler appelait Next(), donc jamais — elle ne protegerait rien.
	r.Get("/me", deps.Guard.Authenticated, deps.Auth.Me)
}

// registerAdminRoutes monte les endpoints de l'admin. Tout est authentifie et
// reserve aux roles internes.
func registerAdminRoutes(r fiber.Router, deps Deps) {
	r.Use(deps.Guard.Authenticated, deps.Guard.RequireRole("admin", "team"))

	// Un endpoint par vue. Exemple du decoupage attendu, a remplacer par les
	// vrais handlers :
	//
	//   GET /admin/dashboard              agregats precalcules (Redis)
	//   GET /admin/projects               liste paginee cote serveur
	//   GET /admin/projects/:id/overview  onglet "Vue d'ensemble" seul
	//   GET /admin/projects/:id/tasks     onglet "Taches" seul
	//
	// RequireRole ci-dessus ne filtre que l'espace : il dit « ce compte est
	// interne ». Le droit precis se verifie par permission, route par route,
	// pour qu'un changement de politique se fasse en base et non dans le code.
	r.Get("/dashboard", todo)
	r.Get("/projects", deps.Guard.RequirePermission("projects.read"), todo)
}

// todo repond 501 tant que le handler n'est pas implemente. Preferable a une
// route absente : le front voit un code d'erreur explicite au lieu d'un 404
// qu'il confondrait avec une ressource manquante.
func todo(c fiber.Ctx) error {
	return &domain.Error{
		Status:  fiber.StatusNotImplemented,
		Code:    "NOT_IMPLEMENTED",
		Message: "Endpoint à implémenter",
		Details: map[string]any{"path": c.Path(), "method": c.Method()},
	}
}
