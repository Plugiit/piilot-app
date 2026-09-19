package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/plugiit-api-go/internal/middleware"
	"github.com/plugiit/plugiit-api-go/internal/security"
)

// testApp monte le routeur complet sans dependance externe : le handler de
// sante n'est pas sollicite par ces tests, seul le routage l'est.
func testApp(t *testing.T) *fiber.App {
	t.Helper()

	app, _ := testAppWith(t, &stubAuth{}, &stubLimiter{})
	return app
}

// testAppWith monte le routeur complet autour d'un service d'authentification
// et d'un compteur donnes, et retourne le signeur pour que les tests puissent
// forger un cookie d'acces valide.
func testAppWith(t *testing.T, auth AuthService, limiter RateLimiter) (*fiber.App, *security.TokenSigner) {
	t.Helper()

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.ErrorHandler(discardLogger()),
	})

	signer := security.NewTokenSigner([]byte(testSecret), "plugiit-api")

	Register(app, Deps{
		Health: NewHealth(nil, nil, BuildInfo{Version: "test"}),
		Auth:   NewAuth(auth, CookieConfig{}, limiter, discardLogger()),
		Guard:  middleware.NewGuard(signer, stubPermissions{granted: true}),
	})

	return app, signer
}

func TestLivenessRepondSansDependance(t *testing.T) {
	app := testApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("statut = %d, attendu %d", res.StatusCode, http.StatusOK)
	}
}

// Garde-fou : toute route sous /api/v1/admin doit exiger une authentification.
// Ce test casse si un endpoint est monte hors du groupe protege.
func TestRoutesAdminExigentUneAuthentification(t *testing.T) {
	app := testApp(t)

	for _, route := range app.GetRoutes(true) {
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		if len(route.Path) < len("/api/v1/admin") || route.Path[:len("/api/v1/admin")] != "/api/v1/admin" {
			continue
		}

		res, err := app.Test(httptest.NewRequest(route.Method, route.Path, nil))
		if err != nil {
			t.Fatalf("%s %s : %v", route.Method, route.Path, err)
		}
		res.Body.Close()

		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s : statut = %d, attendu 401 sans cookie", route.Method, route.Path, res.StatusCode)
		}
	}
}

func TestErreurRetourneLeFormatUnique(t *testing.T) {
	app := testApp(t)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/route-inexistante", nil))
	if err != nil {
		t.Fatalf("requete : %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("statut = %d, attendu 404", res.StatusCode)
	}

	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, fiber.MIMEApplicationJSON) {
		t.Errorf("Content-Type = %q, attendu un JSON", ct)
	}
}
