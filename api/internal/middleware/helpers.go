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

func asFiberError(err error, target **fiber.Error) bool {
	return errors.As(err, target)
}
