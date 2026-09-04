package handler

import (
	"context"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/middleware"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// AuthService est le contrat dont les endpoints d'authentification ont besoin.
//
// Declare ici, chez le consommateur, plutot que d'utiliser directement
// *usecase.AuthService : les handlers deviennent testables sans Postgres, ce
// qui permet de verifier l'ordre des gardes et le format des reponses dans des
// tests rapides.
type AuthService interface {
	Login(ctx context.Context, in usecase.LoginInput) (usecase.Session, error)
	Refresh(ctx context.Context, rawToken, userAgent string, ip *netip.Addr) (usecase.Session, error)
	Logout(ctx context.Context, rawToken string) error
	Me(ctx context.Context, userID uuid.UUID) (usecase.Profile, error)
}

// RateLimiter borne le nombre de tentatives d'authentification.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error)
	Reset(ctx context.Context, key string) error
}

// Bornes des tentatives de connexion.
const (
	// Par IP : large. Toute l'agence peut sortir derriere une seule adresse, et
	// un lundi matin y ressemble a une attaque si le seuil est trop bas.
	loginMaxPerIP = 30
	// Par compte : serre. C'est la borne qui compte, parce qu'une attaque
	// ciblee repartie sur plusieurs adresses passerait sous la limite par IP.
	loginMaxPerAccount = 5

	loginWindow = 15 * time.Minute
)

// Auth porte les endpoints d'authentification.
type Auth struct {
	svc     AuthService
	cookies CookieConfig
	limiter RateLimiter
	log     *slog.Logger
}

// NewAuth construit le handler d'authentification.
func NewAuth(svc AuthService, cookies CookieConfig, limiter RateLimiter, log *slog.Logger) *Auth {
	return &Auth{svc: svc, cookies: cookies, limiter: limiter, log: log}
}

// loginRequest est le corps attendu par POST /auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// sessionResponse est renvoyee par login, refresh et me.
//
// Aucun jeton n'y figure : ils partent en cookies httpOnly. Les exposer aussi
// dans le corps annulerait l'interet du httpOnly, puisque le JavaScript
// pourrait alors les lire et les stocker.
type sessionResponse struct {
	User usecase.Profile `json:"user"`
}

// Login authentifie un compte et ouvre une session.
func (h *Auth) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	// citext rend la comparaison insensible a la casse cote base ; le trim
	// couvre le copier-coller avec une espace parasite.
	email := strings.TrimSpace(req.Email)

	details := map[string]any{}
	if email == "" {
		details["email"] = "L'adresse e-mail est obligatoire"
	}
	if req.Password == "" {
		details["password"] = "Le mot de passe est obligatoire"
	}
	if len(details) > 0 {
		return domain.ErrValidation.WithDetails(details)
	}

	// Verifie avant d'appeler le service : bcrypt coute 250 ms de CPU par
	// tentative, et c'est exactement ce cout qu'une attaque cherche a nous
	// faire payer. Compter apres l'aurait deja fait payer.
	if err := h.checkLoginRate(c, email); err != nil {
		return err
	}

	session, err := h.svc.Login(c.Context(), usecase.LoginInput{
		Email:     email,
		Password:  req.Password,
		UserAgent: c.Get(fiber.HeaderUserAgent),
		IP:        clientIP(c),
	})
	if err != nil {
		return err
	}

	// Connexion reussie : les tentatives ratees d'un utilisateur qui finit par
	// retrouver son mot de passe ne doivent pas le penaliser ensuite.
	h.resetLoginRate(c, email)

	setSession(c, session, h.cookies)

	return c.JSON(sessionResponse{User: session.Profile})
}

// loginRateKeys retourne les compteurs a verifier pour une tentative donnee.
func loginRateKeys(c fiber.Ctx, email string) []struct {
	key   string
	limit int
} {
	return []struct {
		key   string
		limit int
	}{
		{key: "login:ip:" + c.IP(), limit: loginMaxPerIP},
		{key: "login:account:" + strings.ToLower(email), limit: loginMaxPerAccount},
	}
}

// checkLoginRate refuse la tentative si l'un des compteurs deborde.
//
// En cas de panne Redis, la tentative passe (fail-open) : une indisponibilite
// du compteur ne doit pas empecher l'agence de se connecter. Le risque est
// borne — /health/ready exige Redis, donc un Redis mort retire de toute facon
// le conteneur du trafic. L'incident part dans les logs en WARN.
func (h *Auth) checkLoginRate(c fiber.Ctx, email string) error {
	for _, k := range loginRateKeys(c, email) {
		allowed, retryAfter, err := h.limiter.Allow(c.Context(), k.key, k.limit, loginWindow)
		if err != nil {
			h.log.Warn("compteur de tentatives indisponible, tentative autorisee", "error", err)
			continue
		}

		if !allowed {
			// Retry-After en secondes, arrondi au superieur : annoncer 0 ferait
			// retenter immediatement.
			seconds := int(retryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))

			return domain.ErrRateLimited
		}
	}

	return nil
}

// resetLoginRate efface les compteurs apres une connexion reussie.
func (h *Auth) resetLoginRate(c fiber.Ctx, email string) {
	for _, k := range loginRateKeys(c, email) {
		if err := h.limiter.Reset(c.Context(), k.key); err != nil {
			// Sans consequence pour l'appelant, qui est deja authentifie : le
			// compteur expirera de lui-meme.
			h.log.Warn("remise a zero du compteur impossible", "error", err)
		}
	}
}

// Refresh echange le jeton de rafraichissement contre une session neuve.
func (h *Auth) Refresh(c fiber.Ctx) error {
	session, err := h.svc.Refresh(c.Context(), c.Cookies(RefreshCookieName), c.Get(fiber.HeaderUserAgent), clientIP(c))
	if err != nil {
		// La session est finie : effacer les cookies evite que le front boucle
		// sur un rafraichissement avec un jeton mort.
		clearSession(c, h.cookies)
		return err
	}

	setSession(c, session, h.cookies)

	return c.JSON(sessionResponse{User: session.Profile})
}

// Logout revoque la session courante.
//
// Les cookies sont effaces meme si la revocation echoue : le client doit se
// retrouver deconnecte dans tous les cas.
func (h *Auth) Logout(c fiber.Ctx) error {
	err := h.svc.Logout(c.Context(), c.Cookies(RefreshCookieName))

	clearSession(c, h.cookies)

	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Me retourne l'identite de l'appelant, permissions comprises.
func (h *Auth) Me(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	profile, err := h.svc.Me(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(sessionResponse{User: profile})
}

// clientIP extrait l'adresse de l'appelant pour la tracer sur la session.
//
// Retourne nil plutot qu'une valeur douteuse si l'adresse est illisible : la
// colonne est nullable, et une IP fausse vaut moins qu'une IP absente.
func clientIP(c fiber.Ctx) *netip.Addr {
	addr, err := netip.ParseAddr(c.IP())
	if err != nil {
		return nil
	}
	return &addr
}
