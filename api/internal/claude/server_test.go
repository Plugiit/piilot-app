package claude

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/valyala/fasthttp"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

const (
	testResource = "https://auth.example.test/mcp"
	testPRM      = "https://auth.example.test/.well-known/oauth-protected-resource/mcp"
	goodToken    = "jeton-valable"
)

// stubAuth reconnait un seul jeton.
type stubAuth struct {
	caller usecase.OAuthCaller
	err    error
}

func (a stubAuth) Authenticate(_ context.Context, raw string) (usecase.OAuthCaller, bool, error) {
	if a.err != nil {
		return usecase.OAuthCaller{}, false, a.err
	}
	return a.caller, raw == goodToken, nil
}

type stubLimiter struct{}

func (stubLimiter) Allow(context.Context, string, int, time.Duration) (bool, time.Duration, error) {
	return true, 0, nil
}

// stubPermissions : compte actif, toutes permissions.
type stubPermissions struct{ role string }

func (p stubPermissions) HasPermission(context.Context, string, string) (bool, error) {
	return true, nil
}
func (p stubPermissions) ActiveRole(context.Context, uuid.UUID) (string, bool, error) {
	return p.role, true, nil
}

func newTestServer(auth Authenticator) (*Server, *security.TokenSigner) {
	signer := security.NewTokenSigner([]byte(strings.Repeat("s", 32)), "piilot-api")
	s := New(Config{
		Resource: testResource, ResourceMetadata: testPRM, Origins: []string{"https://admin.example.test"},
		Version: "test", Auth: auth, Signer: signer, Limiter: stubLimiter{},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return s, signer
}

func caller(scopes ...string) usecase.OAuthCaller {
	return usecase.OAuthCaller{
		UserID: uuid.New(), Role: "team", GrantID: uuid.New(), Client: "Claude Code",
		Scopes: scopes, ExpiresAt: time.Now().Add(time.Hour),
	}
}

func gateApp(s *Server) *fiber.App {
	app := fiber.New()
	app.Post("/mcp", s.Gate, func(c fiber.Ctx) error { return c.SendString("passe") })
	return app
}

func post(t *testing.T, app *fiber.App, body string, headers map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func TestBarriereSansJetonLanceLaConnexion(t *testing.T) {
	s, _ := newTestServer(stubAuth{caller: caller(usecase.ScopeRead)})
	res := post(t, gateApp(s), `{}`, nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("statut %d, attendu 401", res.StatusCode)
	}
	challenge := res.Header.Get("WWW-Authenticate")
	if !strings.Contains(challenge, `resource_metadata="`+testPRM+`"`) || !strings.Contains(challenge, `scope="`+strings.Join(usecase.OfferedScopes, " ")+`"`) {
		t.Fatalf("defi incomplet : %q", challenge)
	}
	if strings.Contains(challenge, "error=") {
		t.Fatalf("sans jeton, pas de code d'erreur (RFC 6750, 3.1) : %q", challenge)
	}
}

func TestBarriereRefuse(t *testing.T) {
	cases := []struct {
		name    string
		auth    stubAuth
		headers map[string]string
		status  int
	}{
		{"jeton inconnu", stubAuth{caller: caller(usecase.ScopeRead)}, map[string]string{"Authorization": "Bearer autre"}, http.StatusUnauthorized},
		{"autre schema", stubAuth{caller: caller(usecase.ScopeRead)}, map[string]string{"Authorization": "Basic " + goodToken}, http.StatusUnauthorized},
		{"cookie de session", stubAuth{caller: caller(usecase.ScopeRead)}, map[string]string{
			"Authorization": "Bearer " + goodToken, "Cookie": middleware.AccessCookieName + "=x",
		}, http.StatusUnauthorized},
		{"origine etrangere", stubAuth{caller: caller(usecase.ScopeRead)}, map[string]string{
			"Authorization": "Bearer " + goodToken, "Origin": "https://evil.example",
		}, http.StatusForbidden},
		{"compte sans claude.connect", stubAuth{err: usecase.ErrClaudeNotAllowed}, map[string]string{"Authorization": "Bearer " + goodToken}, http.StatusForbidden},
		{"panne", stubAuth{err: errors.New("base indisponible")}, map[string]string{"Authorization": "Bearer " + goodToken}, http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := newTestServer(c.auth)
			if res := post(t, gateApp(s), `{}`, c.headers); res.StatusCode != c.status {
				t.Fatalf("statut %d, attendu %d", res.StatusCode, c.status)
			}
		})
	}
}

func TestBarriereLaissePasserUnJetonValable(t *testing.T) {
	s, _ := newTestServer(stubAuth{caller: caller(usecase.ScopeRead)})
	res := post(t, gateApp(s), `{}`, map[string]string{"Authorization": "Bearer " + goodToken, "Origin": "https://admin.example.test"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("statut %d, attendu 200", res.StatusCode)
	}
}

func TestEcritureSansPorteeDemandeUnStepUp(t *testing.T) {
	s, _ := newTestServer(stubAuth{caller: caller(usecase.ScopeRead)})
	s.kinds["create_task"] = true
	app := gateApp(s)
	headers := map[string]string{"Authorization": "Bearer " + goodToken}

	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_task"}}`,
		// Le SDK lit les cles a la lettre : la barriere aussi.
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","Method":"ping","params":{"name":"create_task"}}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"create_task","NAME":"get_task"}}`,
	} {
		res := post(t, app, body, headers)
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("statut %d, attendu 403 pour %s", res.StatusCode, body)
		}
		if challenge := res.Header.Get("WWW-Authenticate"); !strings.Contains(challenge, `error="insufficient_scope"`) {
			t.Fatalf("defi sans insufficient_scope : %q", challenge)
		}
	}

	// Un outil de lecture passe avec le meme jeton.
	if res := post(t, app, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_task"}}`, headers); res.StatusCode != http.StatusOK {
		t.Fatalf("lecture refusee : %d", res.StatusCode)
	}
}

func TestLaBarriereNeLaissePasserQueCeQuElleLit(t *testing.T) {
	s, _ := newTestServer(stubAuth{caller: caller(usecase.ScopeRead)})
	app := gateApp(s)
	auth := map[string]string{"Authorization": "Bearer " + goodToken}

	cases := []struct {
		name    string
		body    string
		headers map[string]string
		status  int
	}{
		{"lot", `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`, auth, http.StatusBadRequest},
		{"suite apres le message", `{"jsonrpc":"2.0","id":1,"method":"tools/list"} x`, auth, http.StatusBadRequest},
		{"illisible", `{"jsonrpc":`, auth, http.StatusBadRequest},
		{"compresse", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, map[string]string{
			"Authorization": "Bearer " + goodToken, "Content-Encoding": "gzip",
		}, http.StatusUnsupportedMediaType},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if res := post(t, app, c.body, c.headers); res.StatusCode != c.status {
				t.Fatalf("statut %d, attendu %d", res.StatusCode, c.status)
			}
		})
	}
}

// L'outil d'ecriture verifie lui-meme la portee : la barriere n'est qu'une
// commodite, et un jour elle pourrait mal lire un corps.
func TestLOutilDEcritureRefuseUneConnexionEnLecture(t *testing.T) {
	s, _ := newTestServer(stubAuth{})
	s.kinds["create_task"] = true
	called := false
	handler := wrap(s, "create_task", func(context.Context, *session, noIn) (any, error) {
		called = true
		return map[string]any{}, nil
	})
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{
		Extra: map[string]any{"session": &session{caller: caller(usecase.ScopeRead)}},
	}}}
	if _, _, err := handler(context.Background(), req, noIn{}); !errors.Is(err, errWriteScope) || called {
		t.Fatalf("erreur %v, appele %v : l'ecriture doit etre refusee avant l'outil", err, called)
	}
}

func TestUnePaniqueDansUnOutilNeTombePasLeProcessus(t *testing.T) {
	s, _ := newTestServer(stubAuth{})
	handler := wrap(s, "get_task", func(context.Context, *session, noIn) (any, error) {
		panic("boum")
	})
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: &auth.TokenInfo{
		Extra: map[string]any{"session": &session{caller: caller(usecase.ScopeRead)}},
	}}}
	if _, _, err := handler(context.Background(), req, noIn{}); !errors.Is(err, errToolPanic) {
		t.Fatalf("erreur %v, attendu errToolPanic", err)
	}
}

func TestMotifDeRoute(t *testing.T) {
	id := uuid.New()
	r := route{http.MethodGet, "/api/v1/admin/tasks/{id}/comments"}
	got, err := r.build([]uuid.UUID{id})
	if err != nil || got != "/api/v1/admin/tasks/"+id.String()+"/comments" {
		t.Fatalf("build = %q, %v", got, err)
	}
	if _, err := r.build(nil); err == nil {
		t.Fatal("un identifiant manquant doit etre refuse")
	}
	if _, err := r.build([]uuid.UUID{id, id}); err == nil {
		t.Fatal("un identifiant de trop doit etre refuse")
	}
}

// Parcours complet : client MCP, barriere, SDK, outil, rappel interne de la
// route REST avec la garde de session et la permission de l'application.
func TestUnOutilRappelleLaRouteAuNomDuCompte(t *testing.T) {
	who := caller(usecase.ScopeRead)
	s, signer := newTestServer(stubAuth{caller: who})

	app := fiber.New()
	guard := middleware.NewGuard(signer, stubPermissions{role: "team"})
	var seenAgent string
	app.Get("/api/v1/admin/projects/:id", guard.Authenticated, guard.RequirePermission("projects.read"), func(c fiber.Ctx) error {
		seenAgent = c.Get(fiber.HeaderUserAgent)
		userID, _ := middleware.UserIDFrom(c)
		return c.JSON(fiber.Map{"id": c.Params("id"), "viewer": userID.String()})
	})
	app.Post("/mcp", s.Gate, s.Handle())
	s.SetRouter(app.Handler())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = app.Shutdown() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   "http://" + ln.Addr().String() + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{goodToken}, Timeout: 5 * time.Second},
	}, nil)
	if err != nil {
		t.Fatalf("connexion : %v", err)
	}
	defer session.Close()

	project := uuid.New()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_project", Arguments: map[string]any{"project_id": project.String()}})
	if err != nil || res.IsError {
		t.Fatalf("appel : %v %+v", err, res)
	}
	wrapped, _ := res.StructuredContent.(map[string]any)
	if wrapped["avertissement"] == nil {
		t.Fatalf("le resultat doit etre marque comme donnee : %#v", wrapped)
	}
	out, _ := wrapped["donnees"].(map[string]any)
	if out["id"] != project.String() || out["viewer"] != who.UserID.String() {
		t.Fatalf("la route n'a pas vu le bon compte : %#v", out)
	}
	if seenAgent != "Piilot MCP (Claude Code)" {
		t.Fatalf("provenance absente : %q", seenAgent)
	}

	// Un identifiant qui n'est pas un UUID n'atteint pas la route.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_project", Arguments: map[string]any{"project_id": "../users"}})
	if err != nil || !res.IsError {
		t.Fatalf("un identifiant invalide doit echouer : %v %+v", err, res)
	}

	// Une route absente de l'application rend son 404 a Claude, en erreur
	// d'outil, sans casser l'appel.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_task", Arguments: map[string]any{"task_id": uuid.NewString()}})
	if err != nil || !res.IsError {
		t.Fatalf("une route refusee doit remonter en erreur d'outil : %v %+v", err, res)
	}
}

// bearerTransport pose le jeton d'acces sur chaque requete du client MCP.
type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func TestLeRappelNOuvreQueLesRoutesDeclarees(t *testing.T) {
	s, _ := newTestServer(stubAuth{})
	s.SetRouter(func(*fasthttp.RequestCtx) {})
	_, err := s.call(context.Background(), &session{caller: caller(usecase.ScopeRead)},
		request{route: route{http.MethodDelete, "/api/v1/admin/projects/{id}"}, ids: []uuid.UUID{uuid.New()}})
	if !errors.Is(err, errRouteNotAllowed) {
		t.Fatalf("erreur %v, attendu errRouteNotAllowed", err)
	}
}
