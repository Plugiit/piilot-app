package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// OAuthService est ce que le serveur d'autorisation du connecteur Claude
// attend du domaine.
type OAuthService interface {
	Config() usecase.OAuthConfig
	Authorize(ctx context.Context, p usecase.AuthorizeParams) (consentURL, browser string, err error)
	Consent(ctx context.Context, id uuid.UUID, browser string) (usecase.OAuthConsent, error)
	Decide(ctx context.Context, id, userID uuid.UUID, browser string, d usecase.OAuthDecision, ip, userAgent string) (string, error)
	Token(ctx context.Context, p usecase.TokenParams) (usecase.TokenResponse, error)
	RevokeToken(ctx context.Context, raw, clientID string) error
	Connections(ctx context.Context, userID uuid.UUID) ([]usecase.ClaudeConnection, error)
	Disconnect(ctx context.Context, userID, grantID uuid.UUID, ip, userAgent string) error
}

// Bornes par adresse IP. Larges : Claude appelle depuis quelques adresses
// d'Anthropic, partagees entre tous les comptes de l'agence.
const (
	oauthAuthorizeMaxPerIP = 60
	oauthTokenMaxPerIP     = 300
	oauthWindow            = 15 * time.Minute
)

// oauthCookiePath : le cookie qui lie une demande a son navigateur ne part
// qu'avec les appels de l'ecran de consentement.
const oauthCookiePath = "/api/v1/auth/oauth"

// OAuth sert la decouverte, l'autorisation, l'echange de jetons et les
// connexions du compte.
type OAuth struct {
	svc     OAuthService
	limiter RateLimiter
	secure  bool
	log     *slog.Logger
}

// NewOAuth construit le handler. secure : cookie Secure, partout sauf en
// developpement.
func NewOAuth(svc OAuthService, limiter RateLimiter, secure bool, log *slog.Logger) *OAuth {
	return &OAuth{svc: svc, limiter: limiter, secure: secure, log: log}
}

// ResourceMetadata sert les metadonnees de la ressource protegee (RFC 9728) :
// ou trouver le serveur d'autorisation du serveur MCP.
func (h *OAuth) ResourceMetadata(c fiber.Ctx) error {
	cfg := h.svc.Config()
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.JSON(fiber.Map{
		"resource":                 cfg.Resource,
		"resource_name":            "Piilot",
		"authorization_servers":    []string{cfg.Issuer},
		"scopes_supported":         usecase.OfferedScopes,
		"bearer_methods_supported": []string{"header"},
	})
}

// ServerMetadata sert les metadonnees du serveur d'autorisation (RFC 8414).
//
// Clients publics et identites CIMD : c'est la combinaison qui fait choisir a
// Claude son identite publiee, sans enregistrement dynamique.
func (h *OAuth) ServerMetadata(c fiber.Ctx) error {
	cfg := h.svc.Config()
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.JSON(fiber.Map{
		"issuer":                                         cfg.Issuer,
		"authorization_endpoint":                         cfg.Issuer + "/oauth/authorize",
		"token_endpoint":                                 cfg.Issuer + "/oauth/token",
		"revocation_endpoint":                            cfg.Issuer + "/oauth/revoke",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"code_challenge_methods_supported":               []string{"S256"},
		"scopes_supported":                               append(slices.Clone(usecase.OfferedScopes), "offline_access"),
		"client_id_metadata_document_supported":          true,
		"authorization_response_iss_parameter_supported": true,
	})
}

// Authorize recoit Claude dans le navigateur, valide la demande et renvoie
// vers l'ecran de consentement.
func (h *OAuth) Authorize(c fiber.Ctx) error {
	if !h.allow(c, "oauth-authorize:ip:"+c.IP(), oauthAuthorizeMaxPerIP) {
		return domain.ErrRateLimited
	}
	consentURL, browser, err := h.svc.Authorize(c.Context(), usecase.AuthorizeParams{
		ResponseType:        c.Query("response_type"),
		ClientID:            c.Query("client_id"),
		RedirectURI:         c.Query("redirect_uri"),
		State:               c.Query("state"),
		CodeChallenge:       c.Query("code_challenge"),
		CodeChallengeMethod: c.Query("code_challenge_method"),
		Scope:               c.Query("scope"),
		Resource:            c.Query("resource"),
	})
	var oerr *usecase.OAuthError
	if errors.As(err, &oerr) {
		// Adresse de retour validee : l'erreur repart chez le client. Sinon elle
		// s'affiche chez nous, jamais vers une adresse inconnue.
		if back := oerr.ClientRedirect(h.svc.Config().Issuer); back != "" {
			return c.Redirect().Status(http.StatusFound).To(back)
		}
		return c.Redirect().Status(http.StatusFound).To(h.svc.Config().ConsentURL + "?erreur=" + url.QueryEscape(oerr.Code))
	}
	if err != nil {
		return err
	}
	// Ce navigateur, et lui seul, pourra trancher la demande.
	c.Cookie(&fiber.Cookie{
		Name:     usecase.OAuthBrowserCookie,
		Value:    browser,
		Path:     oauthCookiePath,
		MaxAge:   int((10 * time.Minute).Seconds()),
		Secure:   h.secure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	return c.Redirect().Status(http.StatusFound).To(consentURL)
}

// Consent rend la demande que l'ecran de consentement montre.
func (h *OAuth) Consent(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	view, err := h.svc.Consent(c.Context(), id, c.Cookies(usecase.OAuthBrowserCookie))
	if err != nil {
		return err
	}
	return c.JSON(view)
}

type decisionRequest struct {
	Approve bool     `json:"approve"`
	Scopes  []string `json:"scopes"`
}

// Decide enregistre le choix du compte et rend l'adresse ou renvoyer le
// navigateur.
//
// L'origine est exigee et doit etre celle de l'ecran de consentement, sur
// l'espace de connexion : ni une page tierce, ni un autre espace de
// l'application (le portail affiche des textes ecrits par les clients).
func (h *OAuth) Decide(c fiber.Ctx) error {
	if c.Get(fiber.HeaderOrigin) != h.svc.Config().Issuer {
		return domain.ErrForbidden.WithMessage("Origine non admise")
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	var body decisionRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithMessage("Corps de requête invalide")
	}
	redirect, err := h.svc.Decide(c.Context(), id, userID, c.Cookies(usecase.OAuthBrowserCookie),
		usecase.OAuthDecision{Approve: body.Approve, Scopes: body.Scopes},
		strings.Clone(c.IP()), strings.Clone(c.Get(fiber.HeaderUserAgent)))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"redirect_to": redirect})
}

// Token echange un code ou un jeton de renouvellement (RFC 6749, 4.1.3 et 6).
// Corps en application/x-www-form-urlencoded ; erreurs au format OAuth.
func (h *OAuth) Token(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	if !h.allow(c, "oauth-token:ip:"+c.IP(), oauthTokenMaxPerIP) {
		return oauthJSON(c, &usecase.OAuthError{Status: http.StatusTooManyRequests, Code: "slow_down", Description: "Trop de requêtes."})
	}
	resp, err := h.svc.Token(c.Context(), usecase.TokenParams{
		GrantType:    postForm(c, "grant_type"),
		ClientID:     postForm(c, "client_id"),
		Code:         postForm(c, "code"),
		RedirectURI:  postForm(c, "redirect_uri"),
		CodeVerifier: postForm(c, "code_verifier"),
		RefreshToken: postForm(c, "refresh_token"),
		Resource:     postForm(c, "resource"),
		Scope:        postForm(c, "scope"),
	})
	var oerr *usecase.OAuthError
	if errors.As(err, &oerr) {
		return oauthJSON(c, oerr)
	}
	if err != nil {
		h.log.Error("echange de jeton", "error", err)
		return oauthJSON(c, &usecase.OAuthError{Status: http.StatusInternalServerError, Code: "server_error", Description: "Erreur interne."})
	}
	return c.JSON(resp)
}

// Revoke revoque un jeton (RFC 7009). Repond 200 meme pour un jeton inconnu.
func (h *OAuth) Revoke(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	token := postForm(c, "token")
	if token == "" {
		return oauthJSON(c, &usecase.OAuthError{Status: http.StatusBadRequest, Code: "invalid_request", Description: "token manquant."})
	}
	if err := h.svc.RevokeToken(c.Context(), token, postForm(c, "client_id")); err != nil {
		h.log.Error("revocation de jeton", "error", err)
		return oauthJSON(c, &usecase.OAuthError{Status: http.StatusInternalServerError, Code: "server_error", Description: "Erreur interne."})
	}
	return c.SendStatus(http.StatusOK)
}

// Me rend la carte Claude du compte : l'adresse a saisir dans Claude et les
// connexions en cours.
func (h *OAuth) Me(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	conns, err := h.svc.Connections(c.Context(), userID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"mcp_url": h.svc.Config().Resource, "connections": conns})
}

// Disconnect revoque une connexion du compte.
func (h *OAuth) Disconnect(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}
	if err := h.svc.Disconnect(c.Context(), userID, id,
		strings.Clone(c.IP()), strings.Clone(c.Get(fiber.HeaderUserAgent))); err != nil {
		return err
	}
	return c.SendStatus(http.StatusNoContent)
}

// allow borne par cle. Sans compteur, on laisse passer en le disant : comme
// pour la connexion, une panne de Redis ne doit pas couper l'agence.
func (h *OAuth) allow(c fiber.Ctx, key string, limit int) bool {
	ok, retryAfter, err := h.limiter.Allow(c.Context(), key, limit, oauthWindow)
	if err != nil {
		h.log.Warn("compteur indisponible, requete OAuth autorisee", "error", err)
		return true
	}
	if !ok {
		seconds := int(retryAfter.Seconds())
		if seconds < 1 {
			seconds = 1
		}
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))
	}
	return ok
}

// postForm lit un champ du corps, jamais de l'adresse : un code, un
// verificateur ou un jeton dans l'URL finirait dans les journaux du proxy.
func postForm(c fiber.Ctx, key string) string {
	return string(c.Request().PostArgs().Peek(key))
}

func oauthJSON(c fiber.Ctx, e *usecase.OAuthError) error {
	if e.Status == http.StatusUnauthorized {
		c.Set(fiber.HeaderWWWAuthenticate, `Basic realm="piilot"`)
	}
	return c.Status(e.Status).JSON(fiber.Map{"error": e.Code, "error_description": e.Description})
}
