package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// appWithRole monte le routeur avec un compte dont la base dit le role.
func appWithRole(t *testing.T, role string) (*fiber.App, *security.TokenSigner) {
	t.Helper()

	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(discardLogger())})
	signer := security.NewTokenSigner([]byte(testSecret), "piilot-api")

	Register(app, Deps{
		Health: NewHealth(nil, nil, BuildInfo{Version: "test"}),
		Auth:   NewAuth(&stubAuth{}, CookieConfig{}, &stubLimiter{}, discardLogger()),
		Guard:  middleware.NewGuard(signer, stubPermissions{granted: true, role: role}),
	})

	return app, signer
}

func call(t *testing.T, app *fiber.App, signer *security.TokenSigner, role, method, path string) int {
	t.Helper()

	token, err := signer.Sign(uuid.New(), role, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: middleware.AccessCookieName, Value: token})

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s : %v", method, path, err)
	}
	res.Body.Close()

	return res.StatusCode
}

// Toute route du portail exige une session, comme celles du back-office.
func TestRoutesDuPortailExigentUneAuthentification(t *testing.T) {
	app := testApp(t)
	seen := 0

	for _, route := range app.GetRoutes(true) {
		if !strings.HasPrefix(route.Path, "/api/v1/client") || route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		seen++

		res, err := app.Test(httptest.NewRequest(route.Method, route.Path, nil))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()

		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s : statut = %d, attendu 401 sans cookie", route.Method, route.Path, res.StatusCode)
		}
	}

	if seen == 0 {
		t.Fatal("aucune route du portail montee")
	}
}

// Un compte de l'agence n'entre pas dans le portail, meme avec toutes les
// permissions : le groupe filtre sur le role relu en base, pas sur le jeton.
func TestUnCompteInterneNEntrePasDansLePortail(t *testing.T) {
	for _, role := range []string{"admin", "team"} {
		app, signer := appWithRole(t, role)

		for _, path := range []string{"/api/v1/client/projects", "/api/v1/client/projects/" + uuid.NewString()} {
			if got := call(t, app, signer, role, http.MethodGet, path); got != http.StatusForbidden {
				t.Errorf("%s sur %s : statut = %d, attendu 403", role, path, got)
			}
		}
	}
}

// Un compte client n'entre pas dans le back-office, quel que soit le role
// porte par son jeton : c'est la base qui fait foi.
func TestUnCompteClientNEntrePasDansLeBackOffice(t *testing.T) {
	app, signer := appWithRole(t, "client")

	for _, path := range []string{"/api/v1/admin/projects", "/api/v1/admin/deliverables", "/api/v1/admin/me/work"} {
		if got := call(t, app, signer, "admin", http.MethodGet, path); got != http.StatusForbidden {
			t.Errorf("client sur %s : statut = %d, attendu 403", path, got)
		}
	}
}
