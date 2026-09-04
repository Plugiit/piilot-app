package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/security"
)

// AccessCookieName porte le jeton d'acces. Cookie httpOnly plutot que header
// Authorization : le jeton reste inaccessible au JavaScript de l'admin, donc
// une faille XSS ne permet pas de l'exfiltrer.
const AccessCookieName = "plugiit_access"

// claimsKey indexe les claims dans les locals de la requete.
const claimsKey = "auth.claims"

// Guard construit les middlewares d'authentification.
type Guard struct {
	signer *security.TokenSigner
}

// NewGuard construit le garde a partir du signeur de jetons.
func NewGuard(signer *security.TokenSigner) *Guard {
	return &Guard{signer: signer}
}

// Authenticated rejette la requete si le cookie d'acces est absent ou invalide.
func (g *Guard) Authenticated(c fiber.Ctx) error {
	raw := c.Cookies(AccessCookieName)
	if raw == "" {
		return domain.ErrUnauthorized
	}

	claims, err := g.signer.Verify(raw)
	if err != nil {
		return domain.ErrUnauthorized.WithCause(err)
	}

	c.Locals(claimsKey, claims)

	return c.Next()
}

// RequireRole exige l'un des roles donnes. A poser apres Authenticated.
func (g *Guard) RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c fiber.Ctx) error {
		claims, ok := ClaimsFrom(c)
		if !ok {
			return domain.ErrUnauthorized
		}

		if _, ok := allowed[claims.Role]; !ok {
			return domain.ErrForbidden
		}

		return c.Next()
	}
}

// ClaimsFrom lit les claims poses par Authenticated.
func ClaimsFrom(c fiber.Ctx) (*security.Claims, bool) {
	claims, ok := c.Locals(claimsKey).(*security.Claims)
	return claims, ok
}

// UserIDFrom lit l'identifiant de l'appelant authentifie.
func UserIDFrom(c fiber.Ctx) (uuid.UUID, bool) {
	claims, ok := ClaimsFrom(c)
	if !ok {
		return uuid.Nil, false
	}
	return claims.UserID, true
}
