package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

const testIssuer = "https://auth.agence.test"

// stubOAuth enregistre ce que le handler lui transmet et rend ce qu'on lui
// dit de rendre.
type stubOAuth struct {
	authorizeErr  error
	tokenErr      error
	tokenParams   usecase.TokenParams
	decideBrowser string
}

func (s *stubOAuth) Config() usecase.OAuthConfig { return usecase.NewOAuthConfig(testIssuer) }

func (s *stubOAuth) Authorize(context.Context, usecase.AuthorizeParams) (string, string, error) {
	if s.authorizeErr != nil {
		return "", "", s.authorizeErr
	}
	return testIssuer + "/autoriser?demande=d", "jeton-navigateur", nil
}

func (s *stubOAuth) Consent(context.Context, uuid.UUID, string) (usecase.OAuthConsent, error) {
	return usecase.OAuthConsent{}, nil
}

func (s *stubOAuth) Decide(_ context.Context, _, _ uuid.UUID, browser string, _ usecase.OAuthDecision, _, _ string) (string, error) {
	s.decideBrowser = browser
	return "https://claude.ai/api/mcp/auth_callback?code=c", nil
}

func (s *stubOAuth) Token(_ context.Context, p usecase.TokenParams) (usecase.TokenResponse, error) {
	s.tokenParams = p
	if s.tokenErr != nil {
		return usecase.TokenResponse{}, s.tokenErr
	}
	return usecase.TokenResponse{AccessToken: "a", TokenType: "Bearer", ExpiresIn: 3600, RefreshToken: "r", Scope: "piilot:read"}, nil
}

func (s *stubOAuth) RevokeToken(context.Context, string, string) error { return nil }
func (s *stubOAuth) Connections(context.Context, uuid.UUID) ([]usecase.ClaudeConnection, error) {
	return nil, nil
}
func (s *stubOAuth) Disconnect(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return nil
}

func oauthApp(svc *stubOAuth) (*fiber.App, *security.TokenSigner) {
	signer := security.NewTokenSigner([]byte(testSecret), "piilot-api")
	guard := middleware.NewGuard(signer, stubPermissions{granted: true})
	h := NewOAuth(svc, &stubLimiter{}, true, discardLogger())
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(discardLogger())})
	app.Get("/oauth/authorize", h.Authorize)
	app.Post("/oauth/token", h.Token)
	app.Post("/api/v1/auth/oauth/requests/:id/decision", guard.Authenticated, h.Decide)
	return app, signer
}

func sendOAuth(t *testing.T, app *fiber.App, req *http.Request) *http.Response {
	t.Helper()
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func TestAutorisationPoseLeCookieDuNavigateur(t *testing.T) {
	app, _ := oauthApp(&stubOAuth{})
	res := sendOAuth(t, app, httptest.NewRequest(http.MethodGet, "/oauth/authorize", nil))
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != testIssuer+"/autoriser?demande=d" {
		t.Fatalf("statut %d vers %q", res.StatusCode, res.Header.Get("Location"))
	}
	cookie := res.Header.Get("Set-Cookie")
	for _, want := range []string{usecase.OAuthBrowserCookie + "=jeton-navigateur", "path=/api/v1/auth/oauth", "HttpOnly", "secure", "SameSite=Lax"} {
		if !strings.Contains(strings.ToLower(cookie), strings.ToLower(want)) {
			t.Fatalf("cookie %q sans %q", cookie, want)
		}
	}
}

func TestErreurDAutorisationNeSortQueVersUnRetourValide(t *testing.T) {
	// Retour valide : l'erreur repart chez le client, avec state et iss.
	toClient := &usecase.OAuthError{Status: 400, Code: "invalid_scope", Description: "x", RedirectURI: "https://claude.ai/api/mcp/auth_callback", State: "s"}
	app, _ := oauthApp(&stubOAuth{authorizeErr: toClient})
	res := sendOAuth(t, app, httptest.NewRequest(http.MethodGet, "/oauth/authorize", nil))
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != http.StatusFound || loc.Host != "claude.ai" || loc.Query().Get("error") != "invalid_scope" ||
		loc.Query().Get("state") != "s" || loc.Query().Get("iss") != testIssuer {
		t.Fatalf("retour au client : %d %q", res.StatusCode, res.Header.Get("Location"))
	}

	// Retour inconnu : l'erreur reste chez nous.
	app, _ = oauthApp(&stubOAuth{authorizeErr: &usecase.OAuthError{Status: 400, Code: "invalid_client", Description: "x"}})
	res = sendOAuth(t, app, httptest.NewRequest(http.MethodGet, "/oauth/authorize", nil))
	if res.Header.Get("Location") != testIssuer+"/autoriser?erreur=invalid_client" {
		t.Fatalf("erreur hors de chez nous : %q", res.Header.Get("Location"))
	}
}

func TestLeConsentementNeSeDonneQueDepuisLEcranDeConnexion(t *testing.T) {
	svc := &stubOAuth{}
	app, signer := oauthApp(svc)
	token, err := signer.Sign(uuid.New(), "admin", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	decide := func(origin string) *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/requests/"+uuid.NewString()+"/decision",
			strings.NewReader(`{"approve":true,"scopes":["piilot:read"]}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", middleware.AccessCookieName+"="+token+"; "+usecase.OAuthBrowserCookie+"=jeton-navigateur")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		return sendOAuth(t, app, req)
	}

	for _, origin := range []string{"", "https://evil.example", "https://client.agence.test"} {
		if res := decide(origin); res.StatusCode != http.StatusForbidden {
			t.Fatalf("origine %q : statut %d, attendu 403", origin, res.StatusCode)
		}
	}
	res := decide(testIssuer)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("depuis l'emetteur : statut %d", res.StatusCode)
	}
	if svc.decideBrowser != "jeton-navigateur" {
		t.Fatalf("cookie du navigateur non transmis : %q", svc.decideBrowser)
	}
}

func TestJetonLuDansLeCorpsSeulement(t *testing.T) {
	svc := &stubOAuth{}
	app, _ := oauthApp(svc)
	req := httptest.NewRequest(http.MethodPost, "/oauth/token?code=dans-l-url",
		strings.NewReader("grant_type=authorization_code&client_id=c&code_verifier=v"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := sendOAuth(t, app, req)
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("statut %d, cache %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	if svc.tokenParams.Code != "" || svc.tokenParams.GrantType != "authorization_code" || svc.tokenParams.CodeVerifier != "v" {
		t.Fatalf("parametres lus : %+v", svc.tokenParams)
	}
}

func TestErreursDuJetonAuFormatOAuth(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{&usecase.OAuthError{Status: http.StatusBadRequest, Code: "invalid_grant", Description: "x"}, http.StatusBadRequest, `"error":"invalid_grant"`},
		{errors.New("base indisponible"), http.StatusInternalServerError, `"error":"server_error"`},
	}
	for _, c := range cases {
		app, _ := oauthApp(&stubOAuth{tokenErr: c.err})
		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader("grant_type=refresh_token&client_id=c"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := sendOAuth(t, app, req)
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != c.status || !strings.Contains(string(body), c.code) || strings.Contains(string(body), "base indisponible") {
			t.Fatalf("statut %d, corps %s", res.StatusCode, body)
		}
	}
}
