//go:build integration

package usecase_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// Connecteur Claude : le parcours OAuth de bout en bout, contre la base.

const (
	claudeAI       = "https://claude.ai/oauth/mcp-oauth-client-metadata"
	claudeAIReturn = "https://claude.ai/api/mcp/auth_callback"
	claudeCode     = "https://claude.ai/oauth/claude-code-client-metadata"
	testIssuer     = "https://auth.piilot.test"
)

func newOAuth(t *testing.T) (*usecase.OAuthService, *usecase.AuthService, *pgxpool.Pool) {
	t.Helper()
	auth, pool := newService(t)
	return usecase.NewOAuthService(pool, usecase.NewOAuthConfig(testIssuer)), auth, pool
}

type pkce struct{ verifier, challenge string }

func newPKCE(t *testing.T) pkce {
	t.Helper()
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return pkce{verifier: verifier, challenge: base64.RawURLEncoding.EncodeToString(sum[:])}
}

// consent ouvre une demande et rend son identifiant.
func consent(t *testing.T, svc *usecase.OAuthService, client, redirect string, p pkce, scope string) (uuid.UUID, string) {
	t.Helper()
	next, browser, err := svc.Authorize(context.Background(), usecase.AuthorizeParams{
		ResponseType: "code", ClientID: client, RedirectURI: redirect, State: "etat-123",
		CodeChallenge: p.challenge, CodeChallengeMethod: "S256", Scope: scope, Resource: testIssuer + "/mcp",
	})
	if err != nil {
		t.Fatalf("autorisation : %v", err)
	}
	u, err := url.Parse(next)
	if err != nil || !strings.HasPrefix(next, testIssuer+"/autoriser?") {
		t.Fatalf("adresse de consentement inattendue : %q", next)
	}
	id, err := uuid.Parse(u.Query().Get("demande"))
	if err != nil {
		t.Fatalf("demande illisible : %v", err)
	}
	if browser == "" {
		t.Fatal("pas de jeton de navigateur")
	}
	return id, browser
}

// approve consent et rend le code recu par le client.
func approve(t *testing.T, svc *usecase.OAuthService, userID uuid.UUID, client, redirect string, p pkce, scopes ...string) string {
	t.Helper()
	id, browser := consent(t, svc, client, redirect, p, "piilot:read piilot:write offline_access")
	back, err := svc.Decide(context.Background(), id, userID, browser, usecase.OAuthDecision{Approve: true, Scopes: scopes}, "203.0.113.7", "test")
	if err != nil {
		t.Fatalf("decision : %v", err)
	}
	u, _ := url.Parse(back)
	q := u.Query()
	if q.Get("state") != "etat-123" || q.Get("iss") != testIssuer || q.Get("code") == "" {
		t.Fatalf("retour incomplet : %q", back)
	}
	return q.Get("code")
}

func exchange(svc *usecase.OAuthService, client, redirect, code, verifier string) (usecase.TokenResponse, error) {
	return svc.Token(context.Background(), usecase.TokenParams{
		GrantType: "authorization_code", ClientID: client, Code: code, RedirectURI: redirect,
		CodeVerifier: verifier, Resource: testIssuer + "/mcp",
	})
}

func refresh(svc *usecase.OAuthService, client, token string) (usecase.TokenResponse, error) {
	return svc.Token(context.Background(), usecase.TokenParams{GrantType: "refresh_token", ClientID: client, RefreshToken: token})
}

func oauthCode(err error) string {
	var oerr *usecase.OAuthError
	if errors.As(err, &oerr) {
		return oerr.Code
	}
	return ""
}

func authenticated(t *testing.T, svc *usecase.OAuthService, token string) (usecase.OAuthCaller, bool) {
	t.Helper()
	caller, ok, err := svc.Authenticate(context.Background(), token)
	if err != nil && !errors.Is(err, usecase.ErrClaudeNotAllowed) {
		t.Fatalf("authentification : %v", err)
	}
	return caller, ok
}

func TestConnexionDeClaudeDeBoutEnBout(t *testing.T) {
	svc, _, pool := newOAuth(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")
	p := newPKCE(t)

	id, browser := consent(t, svc, claudeAI, claudeAIReturn, p, "")
	if _, err := svc.Consent(ctx, id, "autre-navigateur"); !errors.Is(err, usecase.ErrOAuthRequestExpired) {
		t.Fatalf("une demande ne se lit que dans son navigateur : %v", err)
	}
	if _, err := svc.Decide(ctx, id, userID, "", usecase.OAuthDecision{Approve: true}, "", ""); !errors.Is(err, usecase.ErrOAuthRequestExpired) {
		t.Fatalf("une demande ne se tranche que dans son navigateur : %v", err)
	}
	view, err := svc.Consent(ctx, id, browser)
	if err != nil {
		t.Fatal(err)
	}
	if view.Client != "Claude" || view.Host != "claude.ai" || view.Local || len(view.Scopes) != len(usecase.OfferedScopes) {
		t.Fatalf("ecran de consentement : %+v", view)
	}

	back, err := svc.Decide(ctx, id, userID, browser, usecase.OAuthDecision{Approve: true, Scopes: []string{"piilot:read", "piilot:write"}}, "203.0.113.7", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(ctx, id, userID, browser, usecase.OAuthDecision{Approve: true}, "", ""); !errors.Is(err, usecase.ErrOAuthRequestExpired) {
		t.Fatalf("une demande ne se tranche qu'une fois : %v", err)
	}
	u, _ := url.Parse(back)
	tokens, err := exchange(svc, claudeAI, claudeAIReturn, u.Query().Get("code"), p.verifier)
	if err != nil {
		t.Fatalf("echange : %v", err)
	}
	if tokens.TokenType != "Bearer" || tokens.Scope != strings.Join(usecase.OfferedScopes, " ") || tokens.ExpiresIn != 3600 {
		t.Fatalf("reponse : %+v", tokens)
	}

	caller, ok := authenticated(t, svc, tokens.AccessToken)
	if !ok || caller.UserID != userID || caller.CanWrite() != slices.Contains(usecase.OfferedScopes, usecase.ScopeWrite) || caller.Client != "Claude" {
		t.Fatalf("appelant : %+v %v", caller, ok)
	}
	conns, err := svc.Connections(ctx, userID)
	if err != nil || len(conns) != 1 || conns[0].Client != "Claude" {
		t.Fatalf("connexions : %+v %v", conns, err)
	}

	var audited int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'claude.connected' AND actor_id = $1`, userID).Scan(&audited); err != nil || audited != 1 {
		t.Fatalf("journal : %d %v", audited, err)
	}

	// Renouvellement : nouvelle paire, l'ancien jeton de renouvellement ne sert plus.
	next, err := refresh(svc, claudeAI, tokens.RefreshToken)
	if err != nil {
		t.Fatalf("renouvellement : %v", err)
	}
	if next.RefreshToken == tokens.RefreshToken || next.AccessToken == tokens.AccessToken {
		t.Fatal("le renouvellement doit tirer de nouveaux jetons")
	}

	// Rejoue aussitot : refuse, sans couper (deux renouvellements simultanes).
	if _, err := refresh(svc, claudeAI, tokens.RefreshToken); oauthCode(err) != "invalid_grant" {
		t.Fatalf("rejeu immediat : %v", err)
	}
	if _, ok := authenticated(t, svc, next.AccessToken); !ok {
		t.Fatal("un rejeu dans le delai de grace ne doit pas couper l'autorisation")
	}

	// Rejoue plus tard : vol presume, l'autorisation tombe.
	if _, err := pool.Exec(ctx, `UPDATE oauth_tokens SET revoked_at = now() - interval '1 minute' WHERE revoked_at IS NOT NULL AND kind = 'refresh'
		AND grant_id IN (SELECT id FROM oauth_grants WHERE user_id = $1)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := refresh(svc, claudeAI, tokens.RefreshToken); oauthCode(err) != "invalid_grant" {
		t.Fatalf("rejeu tardif : %v", err)
	}
	if _, ok := authenticated(t, svc, next.AccessToken); ok {
		t.Fatal("un rejeu tardif doit couper l'autorisation")
	}
	if _, err := refresh(svc, claudeAI, next.RefreshToken); oauthCode(err) != "invalid_grant" {
		t.Fatal("plus aucun renouvellement apres la coupure")
	}
}

func TestUnCodeNeSertQuUneFois(t *testing.T) {
	svc, _, pool := newOAuth(t)
	userID, _ := createUser(t, pool, "admin")
	p := newPKCE(t)
	code := approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read")

	tokens, err := exchange(svc, claudeAI, claudeAIReturn, code, p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exchange(svc, claudeAI, claudeAIReturn, code, p.verifier); oauthCode(err) != "invalid_grant" {
		t.Fatalf("second echange : %v", err)
	}
	if _, ok := authenticated(t, svc, tokens.AccessToken); ok {
		t.Fatal("un code rejoue doit couper ce qu'il a ouvert")
	}
}

func TestLeCodeEstLieAuVerificateurEtAuRetour(t *testing.T) {
	svc, _, pool := newOAuth(t)
	userID, _ := createUser(t, pool, "team")
	p := newPKCE(t)

	code := approve(t, svc, userID, claudeCode, "http://127.0.0.1:43123/callback", p, "piilot:read")
	if _, err := exchange(svc, claudeCode, "http://127.0.0.1:43123/callback", code, newPKCE(t).verifier); oauthCode(err) != "invalid_grant" {
		t.Fatalf("autre verificateur : %v", err)
	}

	code = approve(t, svc, userID, claudeCode, "http://localhost:43123/callback", p, "piilot:read")
	if _, err := exchange(svc, claudeCode, "http://localhost:50000/callback", code, p.verifier); oauthCode(err) != "invalid_grant" {
		t.Fatalf("autre retour : %v", err)
	}

	code = approve(t, svc, userID, claudeCode, "http://localhost:43123/callback", p, "piilot:read")
	if _, err := exchange(svc, claudeAI, "http://localhost:43123/callback", code, p.verifier); oauthCode(err) != "invalid_grant" {
		t.Fatalf("autre client : %v", err)
	}
}

func TestDemandesRefusees(t *testing.T) {
	svc, _, _ := newOAuth(t)
	p := newPKCE(t)
	base := usecase.AuthorizeParams{
		ResponseType: "code", ClientID: claudeAI, RedirectURI: claudeAIReturn, State: "s",
		CodeChallenge: p.challenge, CodeChallengeMethod: "S256",
	}
	cases := []struct {
		name       string
		edit       func(*usecase.AuthorizeParams)
		code       string
		redirected bool
	}{
		{"client inconnu", func(a *usecase.AuthorizeParams) { a.ClientID = "https://evil.example/client" }, "invalid_client", false},
		{"retour inconnu", func(a *usecase.AuthorizeParams) { a.RedirectURI = "https://evil.example/cb" }, "invalid_request", false},
		{"sans PKCE", func(a *usecase.AuthorizeParams) { a.CodeChallenge = "" }, "invalid_request", true},
		{"PKCE plain", func(a *usecase.AuthorizeParams) { a.CodeChallengeMethod = "plain" }, "invalid_request", true},
		{"autre type", func(a *usecase.AuthorizeParams) { a.ResponseType = "token" }, "unsupported_response_type", true},
		{"portee inconnue", func(a *usecase.AuthorizeParams) { a.Scope = "admin" }, "invalid_scope", true},
		{"autre ressource", func(a *usecase.AuthorizeParams) { a.Resource = "https://evil.example/mcp" }, "invalid_target", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := base
			c.edit(&params)
			_, _, err := svc.Authorize(context.Background(), params)
			var oerr *usecase.OAuthError
			if !errors.As(err, &oerr) || oerr.Code != c.code {
				t.Fatalf("erreur %v, attendu %s", err, c.code)
			}
			if (oerr.RedirectURI != "") != c.redirected {
				t.Fatalf("renvoi au client = %q, attendu %v", oerr.RedirectURI, c.redirected)
			}
		})
	}
}

func TestEcritureDecocheeLectureSeule(t *testing.T) {
	svc, _, pool := newOAuth(t)
	userID, _ := createUser(t, pool, "team")
	p := newPKCE(t)
	code := approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read")
	tokens, err := exchange(svc, claudeAI, claudeAIReturn, code, p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.Scope != "piilot:read" {
		t.Fatalf("portee : %q", tokens.Scope)
	}
	if caller, ok := authenticated(t, svc, tokens.AccessToken); !ok || caller.CanWrite() {
		t.Fatalf("lecture seule attendue : %+v", caller)
	}
	// Un renouvellement ne s'elargit pas : la portee en trop est ignoree, la
	// connexion continue.
	wider, err := svc.Token(context.Background(), usecase.TokenParams{
		GrantType: "refresh_token", ClientID: claudeAI, RefreshToken: tokens.RefreshToken, Scope: "piilot:read piilot:write",
	})
	if err != nil || wider.Scope != "piilot:read" {
		t.Fatalf("renouvellement elargi : %+v %v", wider, err)
	}
}

func TestUnDroitRetireCoupeClaudeAuProchainAppel(t *testing.T) {
	svc, _, pool := newOAuth(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")
	p := newPKCE(t)
	tokens, err := exchange(svc, claudeAI, claudeAIReturn, approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read"), p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	// Le compte passe a un role sans claude.connect.
	if _, err := pool.Exec(ctx, `UPDATE users SET role = 'client' WHERE id = $1`, userID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Authenticate(ctx, tokens.AccessToken); !errors.Is(err, usecase.ErrClaudeNotAllowed) {
		t.Fatalf("droit retire : %v", err)
	}
	if _, err := refresh(svc, claudeAI, tokens.RefreshToken); oauthCode(err) != "invalid_grant" {
		t.Fatalf("renouvellement sans le droit : %v", err)
	}
}

func TestDesactiverUnCompteRevoqueSesConnexions(t *testing.T) {
	svc, _, pool := newOAuth(t)
	ctx := context.Background()
	admin, _ := createUser(t, pool, "admin")
	userID, _ := createUser(t, pool, "team")
	p := newPKCE(t)
	tokens, err := exchange(svc, claudeAI, claudeAIReturn, approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read"), p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := usecase.NewAccountService(pool, "https://team.piilot.test", false).Disable(ctx, admin, userID); err != nil {
		t.Fatal(err)
	}
	if _, ok := authenticated(t, svc, tokens.AccessToken); ok {
		t.Fatal("un compte desactive ne doit plus rien pouvoir")
	}
	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM oauth_grants WHERE user_id = $1 AND revoked_at IS NULL`, userID).Scan(&live); err != nil || live != 0 {
		t.Fatalf("autorisations encore actives : %d %v", live, err)
	}
}

func TestRefusRenvoieAccessDenied(t *testing.T) {
	svc, _, pool := newOAuth(t)
	userID, _ := createUser(t, pool, "team")
	id, browser := consent(t, svc, claudeAI, claudeAIReturn, newPKCE(t), "")
	back, err := svc.Decide(context.Background(), id, userID, browser, usecase.OAuthDecision{Approve: false}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(back)
	if u.Query().Get("error") != "access_denied" || u.Query().Get("state") != "etat-123" || u.Query().Get("code") != "" {
		t.Fatalf("refus : %q", back)
	}
}

func TestUnCompteClientNObtientPasDeJeton(t *testing.T) {
	svc, _, pool := newOAuth(t)
	userID, _ := createUser(t, pool, "client")
	p := newPKCE(t)
	code := approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read")
	if _, err := exchange(svc, claudeAI, claudeAIReturn, code, p.verifier); oauthCode(err) != "invalid_grant" {
		t.Fatalf("compte client : %v", err)
	}
}

func TestMotDePasseChangeCoupeClaude(t *testing.T) {
	svc, auth, pool := newOAuth(t)
	userID, _ := createUser(t, pool, "team")
	p := newPKCE(t)
	tokens, err := exchange(svc, claudeAI, claudeAIReturn, approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read"), p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ChangePassword(context.Background(), userID, testPassword, "un-autre-mot-de-passe-solide"); err != nil {
		t.Fatal(err)
	}
	if _, ok := authenticated(t, svc, tokens.AccessToken); ok {
		t.Fatal("un mot de passe change doit couper Claude")
	}
}

func TestDeconnexionDepuisMonCompteEtRevocation(t *testing.T) {
	svc, _, pool := newOAuth(t)
	ctx := context.Background()
	userID, _ := createUser(t, pool, "team")
	other, _ := createUser(t, pool, "team")
	p := newPKCE(t)

	first, err := exchange(svc, claudeAI, claudeAIReturn, approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read"), p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	conns, _ := svc.Connections(ctx, userID)
	if len(conns) != 1 {
		t.Fatalf("connexions : %+v", conns)
	}
	if err := svc.Disconnect(ctx, other, conns[0].ID, "", ""); err == nil {
		t.Fatal("un compte ne revoque pas la connexion d'un autre")
	}
	if err := svc.Disconnect(ctx, userID, conns[0].ID, "203.0.113.7", "test"); err != nil {
		t.Fatal(err)
	}
	if _, ok := authenticated(t, svc, first.AccessToken); ok {
		t.Fatal("une connexion revoquee ne vaut plus")
	}
	if conns, _ := svc.Connections(ctx, userID); len(conns) != 0 {
		t.Fatalf("connexion encore listee : %+v", conns)
	}

	// RFC 7009 : revoquer le jeton de renouvellement emporte l'autorisation.
	second, err := exchange(svc, claudeAI, claudeAIReturn, approve(t, svc, userID, claudeAI, claudeAIReturn, p, "piilot:read"), p.verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeToken(ctx, second.RefreshToken, claudeAI); err != nil {
		t.Fatal(err)
	}
	if _, ok := authenticated(t, svc, second.AccessToken); ok {
		t.Fatal("la revocation du jeton de renouvellement doit couper l'autorisation")
	}
	if err := svc.RevokeToken(ctx, "piilot_rt_inconnu", claudeAI); err != nil {
		t.Fatalf("un jeton inconnu n'est pas une erreur : %v", err)
	}
	if err := svc.Purge(ctx); err != nil {
		t.Fatalf("purge : %v", err)
	}
}
