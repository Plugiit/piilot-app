package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Serveur d'autorisation OAuth 2.1 du connecteur Claude.
//
// Claude (claude.ai, Desktop, Claude Code) demande l'acces a un compte ; le
// compte consent ; Claude recoit des jetons qui ne valent que pour le serveur
// MCP de Piilot (/mcp). Les droits restent ceux du compte, relus a chaque
// appel : un jeton n'ajoute rien, il porte seulement le compte.
//
// Seul Claude peut se connecter. Ses deux identites publiques (documents CIMD
// heberges par Anthropic) sont recopiees ici plutot que lues a la volee : pas
// d'appel externe pendant une requete, pas de requete sortante vers une
// adresse fournie par un inconnu.

// Portees. L'ecriture comprend la lecture.
const (
	ScopeRead  = "piilot:read"
	ScopeWrite = "piilot:write"
	// scopeOffline : Claude l'ajoute pour obtenir un jeton de renouvellement.
	// Il n'est pas stocke : les autorisations en recoivent toujours un.
	scopeOffline = "offline_access"
)

// OfferedScopes : les portees que Piilot propose aujourd'hui. L'ecriture n'y
// entrera qu'avec ses outils : une autorisation donnee avant ne l'obtiendra que
// par un nouveau consentement, jamais en silence.
var OfferedScopes = []string{ScopeRead}

// ClaudePermission : le droit de connecter Claude a son compte.
const ClaudePermission = "claude.connect"

const (
	oauthRequestTTL = 10 * time.Minute
	oauthCodeTTL    = 2 * time.Minute
	oauthAccessTTL  = time.Hour
	oauthRefreshTTL = 30 * 24 * time.Hour
	// oauthRefreshGrace (10 s, compte en base dans LockOAuthRefreshToken) : un
	// jeton de renouvellement rejoue dans ce delai est refuse sans couper
	// l'autorisation. Deux renouvellements simultanes du meme client ne sont
	// pas un vol.
	oauthStateMax = 2048
)

// OAuthClient : un client admis. Les adresses de retour sont celles des
// documents CIMD d'Anthropic.
type OAuthClient struct {
	ID        string
	Label     string
	Surface   string
	Redirects []string
}

// claudeClients : les deux identites publiques de Claude.
//
// claude.ai, Claude Desktop et le mobile partagent la premiere ; Claude Code a
// la sienne, avec un retour local sur un port qui change a chaque session.
// Si Anthropic change ses documents, c'est ici qu'il faut les recopier.
var claudeClients = map[string]OAuthClient{
	"https://claude.ai/oauth/mcp-oauth-client-metadata": {
		ID:        "https://claude.ai/oauth/mcp-oauth-client-metadata",
		Label:     "Claude",
		Surface:   "claude.ai, Claude Desktop et mobile",
		Redirects: []string{"https://claude.ai/api/mcp/auth_callback"},
	},
	"https://claude.ai/oauth/claude-code-client-metadata": {
		ID:        "https://claude.ai/oauth/claude-code-client-metadata",
		Label:     "Claude Code",
		Surface:   "Claude Code",
		Redirects: []string{"http://localhost/callback", "http://127.0.0.1/callback"},
	},
}

// LookupOAuthClient rend le client admis sous cet identifiant.
func LookupOAuthClient(id string) (OAuthClient, bool) {
	c, ok := claudeClients[id]
	return c, ok
}

func (c OAuthClient) acceptsRedirect(uri string) bool {
	for _, r := range c.Redirects {
		if security.MatchRedirectURI(r, uri) {
			return true
		}
	}
	return false
}

// OAuthError : erreur du protocole, rendue telle que la RFC la nomme
// (invalid_grant, invalid_request…). Elle ne passe pas par le format
// {code,message} de l'API : Claude lit ces codes-la.
type OAuthError struct {
	Status      int
	Code        string
	Description string
	// RedirectURI et State : renseignes quand l'erreur d'autorisation peut
	// revenir au client, une fois son adresse de retour validee.
	RedirectURI string
	State       string
}

func (e *OAuthError) Error() string { return e.Code + ": " + e.Description }

// ClientRedirect : l'adresse de retour du client, portant l'erreur (RFC 6749,
// 4.1.2.1). Vide quand l'erreur ne doit pas quitter Piilot.
func (e *OAuthError) ClientRedirect(issuer string) string {
	if e.RedirectURI == "" {
		return ""
	}
	params := url.Values{"error": {e.Code}, "error_description": {e.Description}, "iss": {issuer}}
	if e.State != "" {
		params.Set("state", e.State)
	}
	return withQuery(e.RedirectURI, params)
}

func oauthErr(code, description string) *OAuthError {
	return &OAuthError{Status: http.StatusBadRequest, Code: code, Description: description}
}

// ErrClaudeNotAllowed : le compte n'a pas (ou plus) le droit de connecter
// Claude. Refus net, sans relancer de connexion.
var ErrClaudeNotAllowed = &domain.Error{
	Status: http.StatusForbidden, Code: "CLAUDE_NOT_ALLOWED",
	Message: "Ce compte ne peut pas connecter Claude",
}

// ErrOAuthRequestExpired : la demande de connexion n'existe plus.
var ErrOAuthRequestExpired = &domain.Error{
	Status: http.StatusNotFound, Code: "OAUTH_REQUEST_EXPIRED",
	Message: "Cette demande de connexion a expiré. Relancez-la depuis Claude.",
}

// OAuthConfig : ou vivent l'emetteur, la ressource MCP et l'ecran de
// consentement.
type OAuthConfig struct {
	// Issuer : origine du serveur d'autorisation, sans barre finale.
	Issuer string
	// Resource : adresse canonique du serveur MCP, celle que l'on saisit dans
	// Claude. Les jetons ne valent que pour elle.
	Resource string
	// ConsentURL : l'ecran ou le compte consent.
	ConsentURL string
}

// NewOAuthConfig derive la configuration de l'origine de l'espace de
// connexion.
func NewOAuthConfig(origin string) OAuthConfig {
	origin = strings.TrimRight(origin, "/")
	return OAuthConfig{Issuer: origin, Resource: origin + "/mcp", ConsentURL: origin + "/autoriser"}
}

// OAuthService tient les demandes, les autorisations et les jetons.
type OAuthService struct {
	pool *pgxpool.Pool
	q    *db.Queries
	cfg  OAuthConfig
}

// NewOAuthService construit le service.
func NewOAuthService(pool *pgxpool.Pool, cfg OAuthConfig) *OAuthService {
	return &OAuthService{pool: pool, q: db.New(pool), cfg: cfg}
}

// Config rend la configuration, pour les documents de decouverte.
func (s *OAuthService) Config() OAuthConfig { return s.cfg }

// AuthorizeParams : la requete de /oauth/authorize.
type AuthorizeParams struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
	Resource            string
}

// OAuthBrowserCookie : le cookie qui lie une demande au navigateur qui l'a
// lancee.
const OAuthBrowserCookie = "piilot_oauth"

// Authorize valide une demande d'autorisation et la met en attente du
// consentement. Rend l'adresse de l'ecran de consentement, et le jeton a poser
// en cookie dans ce navigateur : seul lui pourra trancher la demande. Un lien
// de consentement transmis a quelqu'un d'autre — « peux-tu valider ma
// connexion ? » — ne lui sert a rien.
//
// Tant que le client et son adresse de retour ne sont pas valides, l'erreur ne
// repart pas vers cette adresse (RFC 6749, 4.1.2.1) : elle s'affiche chez nous.
func (s *OAuthService) Authorize(ctx context.Context, p AuthorizeParams) (consentURL, browser string, err error) {
	client, ok := LookupOAuthClient(p.ClientID)
	if !ok {
		return "", "", oauthErr("invalid_client", "Seul Claude peut se connecter à Piilot.")
	}
	if p.RedirectURI == "" || !client.acceptsRedirect(p.RedirectURI) {
		return "", "", oauthErr("invalid_request", "Adresse de retour inconnue pour ce client.")
	}
	if len(p.State) > oauthStateMax {
		return "", "", oauthErr("invalid_request", "Paramètre state trop long.")
	}

	back := func(code, description string) error {
		e := oauthErr(code, description)
		e.RedirectURI, e.State = p.RedirectURI, p.State
		return e
	}
	if p.ResponseType != "code" {
		return "", "", back("unsupported_response_type", "Seul response_type=code est servi.")
	}
	if p.CodeChallengeMethod != "S256" || !security.ValidPKCEChallenge(p.CodeChallenge) {
		return "", "", back("invalid_request", "PKCE S256 obligatoire.")
	}
	scopes, ok := parseScopes(p.Scope)
	if !ok {
		return "", "", back("invalid_scope", "Portée inconnue.")
	}
	if p.Resource != "" && !s.isResource(p.Resource) {
		return "", "", back("invalid_target", "Ces jetons ne valent que pour "+s.cfg.Resource+".")
	}

	browser, browserHash, err := security.NewOneTimeToken()
	if err != nil {
		return "", "", err
	}
	var state *string
	if p.State != "" {
		state = &p.State
	}
	id, err := s.q.CreateOAuthRequest(ctx, db.CreateOAuthRequestParams{
		ClientID: client.ID, RedirectUri: p.RedirectURI, State: state, CodeChallenge: p.CodeChallenge,
		Scopes: scopes, Resource: s.cfg.Resource, BrowserHash: browserHash, ExpiresAt: time.Now().Add(oauthRequestTTL),
	})
	if err != nil {
		return "", "", fmt.Errorf("enregistrement de la demande : %w", err)
	}
	return s.cfg.ConsentURL + "?demande=" + id.String(), browser, nil
}

// browserHash : l'empreinte du cookie presente, ou rien qui puisse
// correspondre.
func browserHash(raw string) []byte {
	if raw == "" {
		return []byte{}
	}
	return security.HashOneTimeToken(raw)
}

// OAuthConsent : ce que l'ecran de consentement montre.
type OAuthConsent struct {
	ID      uuid.UUID `json:"id"`
	Client  string    `json:"client"`
	Surface string    `json:"surface"`
	// Host : l'hote qui publie l'identite du client (claude.ai), pas le nom
	// qu'il se donne.
	Host string `json:"host"`
	// RedirectHost : ou le code partira. Local : n'importe quel programme de
	// la machine pourrait l'attendre, l'ecran le dit.
	RedirectHost string    `json:"redirect_host"`
	Local        bool      `json:"local"`
	Scopes       []string  `json:"scopes"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Consent rend une demande en attente, au navigateur qui l'a lancee.
func (s *OAuthService) Consent(ctx context.Context, id uuid.UUID, browser string) (OAuthConsent, error) {
	row, err := s.q.GetOAuthRequest(ctx, db.GetOAuthRequestParams{ID: id, BrowserHash: browserHash(browser)})
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthConsent{}, ErrOAuthRequestExpired
	}
	if err != nil {
		return OAuthConsent{}, fmt.Errorf("lecture de la demande : %w", err)
	}
	client, ok := LookupOAuthClient(row.ClientID)
	if !ok {
		return OAuthConsent{}, ErrOAuthRequestExpired
	}
	return OAuthConsent{
		ID: row.ID, Client: client.Label, Surface: client.Surface, Host: hostOf(client.ID),
		RedirectHost: hostOf(row.RedirectUri), Local: security.IsLoopbackRedirect(row.RedirectUri),
		Scopes: row.Scopes, ExpiresAt: row.ExpiresAt,
	}, nil
}

// OAuthDecision : la reponse du compte. Scopes : les portees qu'il accorde,
// parmi celles demandees.
type OAuthDecision struct {
	Approve bool     `json:"approve"`
	Scopes  []string `json:"scopes"`
}

// Decide tranche une demande et rend l'adresse ou renvoyer le navigateur :
// celle du client, avec le code ou le refus. Seul le navigateur qui a lance la
// demande peut la trancher.
func (s *OAuthService) Decide(ctx context.Context, id, userID uuid.UUID, browser string, d OAuthDecision, ip, userAgent string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	req, err := q.ConsumeOAuthRequest(ctx, db.ConsumeOAuthRequestParams{ID: id, BrowserHash: browserHash(browser)})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOAuthRequestExpired
	}
	if err != nil {
		return "", fmt.Errorf("lecture de la demande : %w", err)
	}
	client, ok := LookupOAuthClient(req.ClientID)
	if !ok {
		return "", ErrOAuthRequestExpired
	}
	params := url.Values{"iss": {s.cfg.Issuer}}
	if req.State != nil {
		params.Set("state", *req.State)
	}

	if !d.Approve {
		if err := tx.Commit(ctx); err != nil {
			return "", fmt.Errorf("validation : %w", err)
		}
		params.Set("error", "access_denied")
		params.Set("error_description", "Connexion refusée par le compte.")
		return withQuery(req.RedirectUri, params), nil
	}

	scopes := grantedScopes(req.Scopes, d.Scopes)
	grantID, err := q.CreateOAuthGrant(ctx, db.CreateOAuthGrantParams{UserID: userID, ClientID: client.ID, Scopes: scopes})
	if err != nil {
		return "", fmt.Errorf("creation de l'autorisation : %w", err)
	}
	code, hash, err := security.NewOAuthToken(security.OAuthCodePrefix)
	if err != nil {
		return "", err
	}
	if err := q.CreateOAuthCode(ctx, db.CreateOAuthCodeParams{
		CodeHash: hash, GrantID: grantID, RedirectUri: req.RedirectUri, CodeChallenge: req.CodeChallenge,
		Resource: req.Resource, ExpiresAt: time.Now().Add(oauthCodeTTL),
	}); err != nil {
		return "", fmt.Errorf("creation du code : %w", err)
	}
	if err := auditOAuth(ctx, q, "claude.connected", &userID, grantID, ip, userAgent, map[string]any{
		"client": client.Label, "scopes": scopes,
	}); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("validation : %w", err)
	}

	params.Set("code", code)
	return withQuery(req.RedirectUri, params), nil
}

// TokenParams : la requete de /oauth/token, en form-urlencoded.
type TokenParams struct {
	GrantType    string
	ClientID     string
	Code         string
	RedirectURI  string
	CodeVerifier string
	RefreshToken string
	Resource     string
	Scope        string
}

// TokenResponse : la reponse de /oauth/token (RFC 6749, 5.1).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// Token echange un code ou un jeton de renouvellement contre des jetons.
func (s *OAuthService) Token(ctx context.Context, p TokenParams) (TokenResponse, error) {
	if _, ok := LookupOAuthClient(p.ClientID); !ok {
		return TokenResponse{}, &OAuthError{Status: http.StatusUnauthorized, Code: "invalid_client", Description: "Client inconnu."}
	}
	if p.Resource != "" && !s.isResource(p.Resource) {
		return TokenResponse{}, oauthErr("invalid_target", "Ces jetons ne valent que pour "+s.cfg.Resource+".")
	}
	switch p.GrantType {
	case "authorization_code":
		return s.exchangeCode(ctx, p)
	case "refresh_token":
		return s.refresh(ctx, p)
	default:
		return TokenResponse{}, oauthErr("unsupported_grant_type", "authorization_code ou refresh_token.")
	}
}

func (s *OAuthService) exchangeCode(ctx context.Context, p TokenParams) (TokenResponse, error) {
	invalid := oauthErr("invalid_grant", "Code invalide, expiré ou déjà utilisé.")
	hash, ok := security.OAuthTokenHash(security.OAuthCodePrefix, p.Code)
	if !ok {
		return TokenResponse{}, invalid
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	code, err := q.ConsumeOAuthCode(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		// Un code rejoue trahit une interception : ce qu'il a ouvert se ferme.
		if prior, err := q.GetOAuthCodeGrant(ctx, hash); err == nil && prior.UsedAt != nil {
			revoked, err := q.RevokeOAuthGrant(ctx, prior.GrantID)
			switch {
			case err == nil:
				uid := revoked.UserID
				if err := auditOAuth(ctx, q, "claude.code_reused", &uid, revoked.ID, "", "", map[string]any{
					"client": clientLabel(revoked.ClientID),
				}); err != nil {
					return TokenResponse{}, err
				}
			case !errors.Is(err, pgx.ErrNoRows):
				return TokenResponse{}, fmt.Errorf("revocation apres un code rejoue : %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return TokenResponse{}, fmt.Errorf("validation : %w", err)
			}
		}
		return TokenResponse{}, invalid
	}
	if err != nil {
		return TokenResponse{}, fmt.Errorf("lecture du code : %w", err)
	}
	if code.RedirectUri != p.RedirectURI || !security.VerifyPKCE(code.CodeChallenge, p.CodeVerifier) {
		return TokenResponse{}, invalid
	}
	grant, err := q.GetOAuthGrant(ctx, code.GrantID)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("lecture de l'autorisation : %w", err)
	}
	if grant.ClientID != p.ClientID || grant.RevokedAt != nil || !grant.Active {
		return TokenResponse{}, invalid
	}
	if err := s.requirePermission(ctx, q, grant.Role); err != nil {
		return TokenResponse{}, invalid
	}

	resp, err := s.issue(ctx, q, grant.ID, code.Resource, grant.Scopes)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TokenResponse{}, fmt.Errorf("validation : %w", err)
	}
	return resp, nil
}

func (s *OAuthService) refresh(ctx context.Context, p TokenParams) (TokenResponse, error) {
	invalid := oauthErr("invalid_grant", "Jeton de renouvellement invalide. Reconnectez Claude.")
	hash, ok := security.OAuthTokenHash(security.OAuthRefreshPrefix, p.RefreshToken)
	if !ok {
		return TokenResponse{}, invalid
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	token, err := q.LockOAuthRefreshToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenResponse{}, invalid
	}
	if err != nil {
		return TokenResponse{}, fmt.Errorf("lecture du jeton : %w", err)
	}
	if token.Used {
		// Deja servi. Au-dela d'un court delai (oauthRefreshGrace, compte en
		// base), c'est un vol : l'autorisation entiere tombe, le compte
		// reconnectera Claude.
		if token.Replayed {
			if revoked, err := q.RevokeOAuthGrant(ctx, token.GrantID); err == nil {
				uid := revoked.UserID
				if err := auditOAuth(ctx, q, "claude.refresh_reused", &uid, revoked.ID, "", "", map[string]any{
					"client": clientLabel(revoked.ClientID),
				}); err != nil {
					return TokenResponse{}, err
				}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return TokenResponse{}, fmt.Errorf("revocation apres un jeton rejoue : %w", err)
			}
			if err := tx.Commit(ctx); err != nil {
				return TokenResponse{}, fmt.Errorf("validation : %w", err)
			}
		}
		return TokenResponse{}, invalid
	}
	if !token.Live {
		return TokenResponse{}, invalid
	}
	grant, err := q.GetOAuthGrant(ctx, token.GrantID)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("lecture de l'autorisation : %w", err)
	}
	if grant.ClientID != p.ClientID || grant.RevokedAt != nil || !grant.Active {
		return TokenResponse{}, invalid
	}
	if err := s.requirePermission(ctx, q, grant.Role); err != nil {
		return TokenResponse{}, invalid
	}
	// Le nouveau jeton ne depasse ni l'ancien ni l'autorisation (RFC 6749, 6).
	// Une portee demandee en plus est simplement absente de la reponse : le
	// client le lit dans « scope », la connexion continue.
	scopes := intersect(token.Scopes, grant.Scopes)
	if p.Scope != "" {
		asked, ok := parseScopes(p.Scope)
		if !ok {
			return TokenResponse{}, oauthErr("invalid_scope", "Portée inconnue.")
		}
		scopes = intersect(asked, scopes)
	}

	if err := q.RevokeOAuthToken(ctx, hash); err != nil {
		return TokenResponse{}, fmt.Errorf("rotation du jeton : %w", err)
	}
	resp, err := s.issue(ctx, q, grant.ID, token.Resource, scopes)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TokenResponse{}, fmt.Errorf("validation : %w", err)
	}
	return resp, nil
}

// issue tire une paire de jetons pour une autorisation.
func (s *OAuthService) issue(ctx context.Context, q *db.Queries, grantID uuid.UUID, resource string, scopes []string) (TokenResponse, error) {
	access, accessHash, err := security.NewOAuthToken(security.OAuthAccessPrefix)
	if err != nil {
		return TokenResponse{}, err
	}
	refresh, refreshHash, err := security.NewOAuthToken(security.OAuthRefreshPrefix)
	if err != nil {
		return TokenResponse{}, err
	}
	now := time.Now()
	for _, t := range []db.CreateOAuthTokenParams{
		{TokenHash: accessHash, GrantID: grantID, Kind: "access", Resource: resource, Scopes: scopes, ExpiresAt: now.Add(oauthAccessTTL)},
		{TokenHash: refreshHash, GrantID: grantID, Kind: "refresh", Resource: resource, Scopes: scopes, ExpiresAt: now.Add(oauthRefreshTTL)},
	} {
		if err := q.CreateOAuthToken(ctx, t); err != nil {
			return TokenResponse{}, fmt.Errorf("creation du jeton : %w", err)
		}
	}
	return TokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int(oauthAccessTTL.Seconds()),
		RefreshToken: refresh, Scope: strings.Join(scopes, " "),
	}, nil
}

// RevokeToken revoque un jeton (RFC 7009). Un jeton de renouvellement emporte
// toute l'autorisation. Un jeton inconnu n'est pas une erreur : la reponse ne
// dit rien de ce qui existe.
func (s *OAuthService) RevokeToken(ctx context.Context, raw, clientID string) error {
	prefix := security.OAuthAccessPrefix
	if strings.HasPrefix(raw, security.OAuthRefreshPrefix) {
		prefix = security.OAuthRefreshPrefix
	}
	hash, ok := security.OAuthTokenHash(prefix, raw)
	if !ok {
		return nil
	}
	row, err := s.q.GetOAuthToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lecture du jeton : %w", err)
	}
	if clientID != "" && clientID != row.ClientID {
		return nil
	}
	if row.Kind == "access" {
		if err := s.q.RevokeOAuthToken(ctx, hash); err != nil {
			return fmt.Errorf("revocation du jeton : %w", err)
		}
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	revoked, err := q.RevokeOAuthGrant(ctx, row.GrantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("revocation de l'autorisation : %w", err)
	}
	uid := revoked.UserID
	if err := auditOAuth(ctx, q, "claude.disconnected", &uid, revoked.ID, "", "", map[string]any{
		"client": clientLabel(revoked.ClientID), "by": "client",
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("validation : %w", err)
	}
	return nil
}

// OAuthCaller : le compte au nom duquel Claude appelle /mcp.
type OAuthCaller struct {
	UserID    uuid.UUID
	Role      string
	GrantID   uuid.UUID
	ClientID  string
	Client    string
	Scopes    []string
	ExpiresAt time.Time
}

// CanWrite dit si l'autorisation porte l'ecriture.
func (c OAuthCaller) CanWrite() bool { return slices.Contains(c.Scopes, ScopeWrite) }

// Authenticate dit au nom de qui un jeton d'acces appelle. Faux : jeton
// inconnu, expire, revoque, emis pour une autre ressource, ou compte
// desactive. ErrClaudeNotAllowed : le compte a perdu le droit.
func (s *OAuthService) Authenticate(ctx context.Context, raw string) (OAuthCaller, bool, error) {
	hash, ok := security.OAuthTokenHash(security.OAuthAccessPrefix, raw)
	if !ok {
		return OAuthCaller{}, false, nil
	}
	row, err := s.q.AuthenticateOAuthAccess(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthCaller{}, false, nil
	}
	if err != nil {
		return OAuthCaller{}, false, fmt.Errorf("lecture du jeton : %w", err)
	}
	if !s.isResource(row.Resource) {
		return OAuthCaller{}, false, nil
	}
	if err := s.requirePermission(ctx, s.q, row.Role); err != nil {
		return OAuthCaller{}, false, err
	}
	if err := s.q.TouchOAuthGrant(ctx, row.GrantID); err != nil {
		return OAuthCaller{}, false, fmt.Errorf("date d'usage : %w", err)
	}
	return OAuthCaller{
		UserID: row.UserID, Role: row.Role, GrantID: row.GrantID, ClientID: row.ClientID,
		Client: clientLabel(row.ClientID), Scopes: row.Scopes, ExpiresAt: row.ExpiresAt,
	}, true, nil
}

// ClaudeConnection : une autorisation vivante, pour Mon compte.
type ClaudeConnection struct {
	ID         uuid.UUID  `json:"id"`
	Client     string     `json:"client"`
	Surface    string     `json:"surface"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// Connections rend les autorisations vivantes d'un compte.
func (s *OAuthService) Connections(ctx context.Context, userID uuid.UUID) ([]ClaudeConnection, error) {
	rows, err := s.q.ListUserOAuthGrants(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("lecture des connexions : %w", err)
	}
	out := make([]ClaudeConnection, 0, len(rows))
	for _, r := range rows {
		c, _ := LookupOAuthClient(r.ClientID)
		out = append(out, ClaudeConnection{
			ID: r.ID, Client: clientLabel(r.ClientID), Surface: c.Surface, Scopes: r.Scopes,
			CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt,
		})
	}
	return out, nil
}

// Disconnect revoque une autorisation du compte. Elle cesse des la requete
// suivante de Claude.
func (s *OAuthService) Disconnect(ctx context.Context, userID, grantID uuid.UUID, ip, userAgent string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.RevokeUserOAuthGrant(ctx, db.RevokeUserOAuthGrantParams{ID: grantID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound.WithMessage("Aucune connexion active avec cet identifiant")
	}
	if err != nil {
		return fmt.Errorf("revocation de la connexion : %w", err)
	}
	if err := auditOAuth(ctx, q, "claude.disconnected", &userID, row.ID, ip, userAgent, map[string]any{
		"client": clientLabel(row.ClientID), "by": "account",
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("validation : %w", err)
	}
	return nil
}

// Purge efface les demandes, codes et jetons expires, et les autorisations
// qui n'ont plus rien a couper.
func (s *OAuthService) Purge(ctx context.Context) error {
	if _, err := s.q.DeleteExpiredOAuthRequests(ctx); err != nil {
		return fmt.Errorf("purge des demandes : %w", err)
	}
	if _, err := s.q.DeleteExpiredOAuthCodes(ctx); err != nil {
		return fmt.Errorf("purge des codes : %w", err)
	}
	if _, err := s.q.DeleteExpiredOAuthTokens(ctx); err != nil {
		return fmt.Errorf("purge des jetons : %w", err)
	}
	if _, err := s.q.DeleteDeadOAuthGrants(ctx); err != nil {
		return fmt.Errorf("purge des autorisations : %w", err)
	}
	return nil
}

func (s *OAuthService) requirePermission(ctx context.Context, q *db.Queries, role string) error {
	granted, err := q.RoleHasPermission(ctx, db.RoleHasPermissionParams{RoleCode: role, PermissionCode: ClaudePermission})
	if err != nil {
		return fmt.Errorf("verification de la permission : %w", err)
	}
	if !granted {
		return ErrClaudeNotAllowed
	}
	return nil
}

// isResource compare une ressource a l'adresse canonique du serveur MCP :
// schema et hote sans casse, sans barre finale (RFC 8707).
func (s *OAuthService) isResource(raw string) bool {
	return canonicalResource(raw) == canonicalResource(s.cfg.Resource)
}

func canonicalResource(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Fragment != "" {
		return ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}

// parseScopes lit une liste de portees separees par des espaces. Vide :
// lecture et ecriture, l'ecran de consentement permet de retirer l'ecriture.
// La lecture est toujours comprise.
func parseScopes(raw string) ([]string, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return slices.Clone(OfferedScopes), true
	}
	out := []string{ScopeRead}
	for _, f := range fields {
		switch f {
		case ScopeRead, scopeOffline:
		case ScopeWrite:
			// Demandee avant d'etre proposee : ignoree, l'ecran n'offre que
			// ce qui existe.
			if slices.Contains(OfferedScopes, ScopeWrite) && !slices.Contains(out, ScopeWrite) {
				out = append(out, ScopeWrite)
			}
		default:
			return nil, false
		}
	}
	return out, true
}

// intersect : les portees demandees que l'on detient, lecture comprise.
func intersect(asked, held []string) []string {
	out := []string{ScopeRead}
	for _, s := range asked {
		if s != ScopeRead && slices.Contains(held, s) {
			out = append(out, s)
		}
	}
	return out
}

// grantedScopes : ce que le compte accorde, borne par ce qui etait demande.
func grantedScopes(requested, chosen []string) []string {
	out := []string{ScopeRead}
	if slices.Contains(requested, ScopeWrite) && slices.Contains(chosen, ScopeWrite) {
		out = append(out, ScopeWrite)
	}
	return out
}

func clientLabel(id string) string {
	if c, ok := LookupOAuthClient(id); ok {
		return c.Label
	}
	return "Client inconnu"
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func withQuery(base string, params url.Values) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + params.Encode()
}

// auditOAuth ecrit au journal, dans la transaction du geste.
func auditOAuth(ctx context.Context, q *db.Queries, action string, actor *uuid.UUID, grantID uuid.UUID, ip, userAgent string, details map[string]any) error {
	payload, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("details du journal : %w", err)
	}
	email := ""
	if actor != nil {
		if u, err := q.GetUserByID(ctx, *actor); err == nil {
			email = u.Email
		}
	}
	if err := q.InsertAudit(ctx, db.InsertAuditParams{
		ActorID: actor, ActorEmail: email, Action: action, TargetType: "claude_connection", TargetID: grantID.String(),
		Ip: ip, UserAgent: truncate(userAgent, 300), Details: payload,
	}); err != nil {
		return fmt.Errorf("journal d'audit : %w", err)
	}
	return nil
}
