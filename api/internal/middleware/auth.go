package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// AccessCookieName porte le jeton d'acces. Cookie httpOnly plutot que header
// Authorization : le jeton reste inaccessible au JavaScript de l'admin, donc
// une faille XSS ne permet pas de l'exfiltrer.
const AccessCookieName = "piilot_access"

// claimsKey indexe les claims dans les locals de la requete.
const claimsKey = "auth.claims"

// guardKey indexe le garde lui-meme, pour que Can puisse interroger les
// permissions depuis un handler sans qu'on lui passe le garde.
const guardKey = "auth.guard"

// PermissionChecker repond a « ce role detient-il cette permission » et a « ce
// compte est-il toujours actif, avec quel role ».
//
// L'interface est declaree ici, chez le consommateur, et non dans le paquet qui
// l'implemente : le middleware n'a ainsi aucune dependance vers usecase ni vers
// le schema SQL.
type PermissionChecker interface {
	HasPermission(ctx context.Context, roleCode, permissionCode string) (bool, error)
	ActiveRole(ctx context.Context, userID uuid.UUID) (role string, active bool, err error)
}

// BotKeyChecker repond a « cette cle du bot est-elle active, avec quel role ».
// Declare ici pour la meme raison que PermissionChecker.
type BotKeyChecker interface {
	AuthenticateBotKey(ctx context.Context, raw string) (role string, keyID uuid.UUID, ok bool, err error)
}

// botCallerKey indexe l'appelant machine pose par Bot.
const botCallerKey = "auth.bot"

// BotCaller est un appel du bot : la cle presentee et le role qu'elle porte.
// Pas de compte derriere — UserIDFrom reste vide, et le journal d'audit ne
// prete le geste a personne.
type BotCaller struct {
	KeyID uuid.UUID
	Role  string
}

// Guard construit les middlewares d'authentification et d'autorisation.
type Guard struct {
	signer      *security.TokenSigner
	permissions PermissionChecker
	botKeys     BotKeyChecker
}

// NewGuard construit le garde a partir du signeur et du verificateur de
// permissions.
func NewGuard(signer *security.TokenSigner, permissions PermissionChecker) *Guard {
	return &Guard{signer: signer, permissions: permissions}
}

// SetBotKeys branche la verification des cles du bot. Sans elle, Bot refuse
// tout appel.
func (g *Guard) SetBotKeys(keys BotKeyChecker) { g.botKeys = keys }

// Bot authentifie un appel machine du bot : « Authorization: Bearer <cle> ».
//
// Cloisonnement dans les deux sens. Une cle n'est lue qu'ici, donc seulement
// sous /api/v1/bot : Authenticated ignore l'en-tete Authorization. Et une
// session n'entre pas ici : le bot appelle de serveur a serveur et n'a jamais
// de cookie ; un cookie de session signe un navigateur, qui n'a rien a faire
// sur ces routes, meme muni d'une cle.
func (g *Guard) Bot(c fiber.Ctx) error {
	if c.Cookies(AccessCookieName) != "" {
		return domain.ErrUnauthorized
	}
	if g.botKeys == nil {
		return domain.ErrUnauthorized
	}
	raw, ok := strings.CutPrefix(c.Get(fiber.HeaderAuthorization), "Bearer ")
	if !ok || raw == "" {
		return domain.ErrUnauthorized
	}

	role, keyID, ok, err := g.botKeys.AuthenticateBotKey(c.Context(), raw)
	if err != nil {
		// Une panne de lecture ne doit pas ouvrir l'acces : on refuse.
		return domain.ErrInternal.WithCause(err)
	}
	if !ok {
		return domain.ErrUnauthorized
	}

	c.Locals(botCallerKey, &BotCaller{KeyID: keyID, Role: role})
	c.Locals(guardKey, g)

	return c.Next()
}

// BotCallerFrom lit l'appelant machine pose par Bot.
func BotCallerFrom(c fiber.Ctx) (*BotCaller, bool) {
	caller, ok := c.Locals(botCallerKey).(*BotCaller)
	return caller, ok
}

// roleFrom rend le role de l'appelant, session ou cle du bot. Une requete ne
// passe que par l'une des deux gardes : il n'y a jamais les deux.
func roleFrom(c fiber.Ctx) (string, bool) {
	if claims, ok := ClaimsFrom(c); ok {
		return claims.Role, true
	}
	if caller, ok := BotCallerFrom(c); ok {
		return caller.Role, true
	}
	return "", false
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

	// Le jeton dit qui appelle ; la base dit si ce compte a encore le droit
	// d'appeler, et avec quel role. Un compte desactive perd l'acces a la
	// requete suivante, un role change s'applique aussitot — sans attendre les
	// quinze minutes du jeton. Une lecture par cle primaire.
	role, active, err := g.permissions.ActiveRole(c.Context(), claims.UserID)
	if err != nil {
		// Une panne de lecture ne doit pas ouvrir l'acces : on refuse.
		return domain.ErrInternal.WithCause(err)
	}
	if !active {
		return domain.ErrSessionExpired
	}

	current := *claims
	current.Role = role
	c.Locals(claimsKey, &current)
	c.Locals(guardKey, g)

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
		role, ok := roleFrom(c)
		if !ok {
			return domain.ErrUnauthorized
		}

		granted, err := g.permissions.HasPermission(c.Context(), role, permission)
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

// Can dit si l'appelant detient une permission, sans refuser la requete.
//
// Pour les ecrans dont une partie seulement depend d'un droit : la liste des
// projets se lit avec `projects.read`, ses heures vendues avec `budgets.read`.
// Refuser toute la liste pour un champ serait trop ; le montrer a tous, faux.
func Can(c fiber.Ctx, permission string) (bool, error) {
	role, ok := roleFrom(c)
	if !ok {
		return false, domain.ErrUnauthorized
	}

	g, ok := c.Locals(guardKey).(*Guard)
	if !ok {
		return false, domain.ErrUnauthorized
	}

	granted, err := g.permissions.HasPermission(c.Context(), role, permission)
	if err != nil {
		return false, domain.ErrInternal.WithCause(err)
	}

	return granted, nil
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
