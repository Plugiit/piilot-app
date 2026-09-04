package middleware

import (
	"context"

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

// PermissionChecker repond a « ce role detient-il cette permission ».
//
// L'interface est declaree ici, chez le consommateur, et non dans le paquet qui
// l'implemente : le middleware n'a ainsi aucune dependance vers usecase ni vers
// le schema SQL.
type PermissionChecker interface {
	HasPermission(ctx context.Context, roleCode, permissionCode string) (bool, error)
}

// Guard construit les middlewares d'authentification et d'autorisation.
type Guard struct {
	signer      *security.TokenSigner
	permissions PermissionChecker
}

// NewGuard construit le garde a partir du signeur et du verificateur de
// permissions.
func NewGuard(signer *security.TokenSigner, permissions PermissionChecker) *Guard {
	return &Guard{signer: signer, permissions: permissions}
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

// RequirePermission exige une permission precise. A poser apres Authenticated.
//
// La permission est relue en base a chaque requete plutot que portee par le
// jeton : retirer un droit prend effet immediatement, sans attendre les 15
// minutes d'expiration de l'acces. Le cout est une lecture indexee sur trois
// tables de reference, negligeable devant la requete metier qui suit.
//
// A preferer a RequireRole des qu'un droit est en jeu : RequireRole code en dur
// dans les routes une correspondance role/droit que le RBAC a justement pour
// role de rendre modifiable sans redeploiement.
func (g *Guard) RequirePermission(permission string) fiber.Handler {
	return func(c fiber.Ctx) error {
		claims, ok := ClaimsFrom(c)
		if !ok {
			return domain.ErrUnauthorized
		}

		granted, err := g.permissions.HasPermission(c.Context(), claims.Role, permission)
		if err != nil {
			// Une panne de lecture ne doit pas ouvrir l'acces : on refuse.
			return domain.ErrInternal.WithCause(err)
		}

		if !granted {
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
