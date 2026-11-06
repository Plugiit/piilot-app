package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// AllowedMethods liste les verbes exposes a l'admin.
var AllowedMethods = []string{
	fiber.MethodGet,
	fiber.MethodPost,
	fiber.MethodPatch,
	fiber.MethodPut,
	fiber.MethodDelete,
	fiber.MethodOptions,
}

// requestID retourne l'identifiant de correlation de la requete courante, pose
// par le middleware requestid. Present dans chaque log pour relier une erreur
// remontee par un utilisateur a sa trace serveur.
func requestID(c fiber.Ctx) string {
	return requestid.FromContext(c)
}

// routePattern rend le motif de la route servie (« /api/v1/bot/discord-links/
// :discord_user_id »), jamais le chemin recu : ses segments portent des
// identifiants Discord, des jetons d'invitation ou de reinitialisation, qui
// n'ont rien a faire dans un journal.
func routePattern(c fiber.Ctx) string {
	if route := c.Route(); route != nil {
		return route.Path
	}
	return ""
}

func asFiberError(err error, target **fiber.Error) bool {
	return errors.As(err, target)
}
