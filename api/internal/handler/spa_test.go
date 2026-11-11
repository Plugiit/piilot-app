package handler

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/spaces"
)

func newSPAApp(t *testing.T) *fiber.App {
	t.Helper()
	return newSPAAppWith(t, spaces.Spaces{})
}

func newSPAAppWith(t *testing.T, sp spaces.Spaces) *fiber.App {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><head></head>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app-abc.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := fiber.New()
	app.Get("/health/live", func(c fiber.Ctx) error { return c.SendString("live") })
	if err := RegisterSPA(app, dir, sp); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestSPA(t *testing.T) {
	app := newSPAApp(t)

	cases := []struct {
		name, path  string
		status      int
		body, cache string
	}{
		{"route d'API existante prioritaire", "/health/live", 200, "live", ""},
		{"racine", "/", 200, "<html><head></head>spa</html>", "no-cache"},
		{"route cliente profonde", "/admin/projects/42", 200, "<html><head></head>spa</html>", "no-cache"},
		{"asset hashe", "/assets/app-abc.js", 200, "console.log(1)", "public, max-age=31536000, immutable"},
		{"route d'API inconnue", "/api/v1/nope", 404, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest("GET", tc.path, nil))
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tc.status {
				t.Fatalf("statut = %d, attendu %d", resp.StatusCode, tc.status)
			}
			if tc.body != "" && string(body) != tc.body {
				t.Fatalf("corps = %q, attendu %q", body, tc.body)
			}
			if tc.status == 404 && strings.Contains(string(body), "<html>") {
				t.Fatal("une route d'API inconnue ne doit pas rendre le front")
			}
			if got := resp.Header.Get(fiber.HeaderCacheControl); tc.cache != "" && got != tc.cache {
				t.Fatalf("Cache-Control = %q, attendu %q", got, tc.cache)
			}
		})
	}
}

func TestSPAWithoutIndex(t *testing.T) {
	if err := RegisterSPA(fiber.New(), t.TempDir(), spaces.Spaces{}); err == nil {
		t.Fatal("un STATIC_DIR sans index.html doit faire echouer le demarrage")
	}
}

// En multi-domaines, une page ouverte sur le mauvais domaine part vers le bon,
// et index.html porte les domaines pour que le front fasse de meme.
func TestSPAMultiDomaines(t *testing.T) {
	sp, err := spaces.Parse("https://auth.agence.fr", "https://team.agence.fr", "https://admin.agence.fr", "https://client.agence.fr", "")
	if err != nil {
		t.Fatal(err)
	}
	app := newSPAAppWith(t, sp)

	req := httptest.NewRequest("GET", "/parametres/comptes?onglet=roles", nil)
	req.Host = "team.agence.fr"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "https://admin.agence.fr/parametres/comptes?onglet=roles" {
		t.Fatalf("redirection : %d vers %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	for _, path := range []string{"/", "/pm/projets"} {
		req = httptest.NewRequest("GET", path, nil)
		req.Host = "team.agence.fr"
		resp, err = app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 || !strings.Contains(string(body), `name="piilot-spaces"`) || !strings.Contains(string(body), "https://admin.agence.fr") {
			t.Fatalf("%s : %d, sans les domaines : %s", path, resp.StatusCode, body)
		}
	}

	// Les fichiers du build et l'API restent servis sur tous les domaines.
	req = httptest.NewRequest("GET", "/assets/app-abc.js", nil)
	req.Host = "client.agence.fr"
	if resp, _ = app.Test(req); resp.StatusCode != 200 {
		t.Fatalf("asset : %d", resp.StatusCode)
	}
}
