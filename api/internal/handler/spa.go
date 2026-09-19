package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// RegisterSPA sert le build du front depuis dir, apres toutes les routes de
// l'API : une route d'API l'emporte toujours, le front ne recoit que le reste.
//
// Le routage est cote client : une URL qui ne designe aucun fichier rend
// index.html, et TanStack Router affiche l'ecran. Sauf sous /api/ — une route
// d'API inconnue doit rester un 404 JSON, pas une page HTML qu'un appelant
// essaierait de decoder.
func RegisterSPA(app *fiber.App, dir string) error {
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return fmt.Errorf("front introuvable dans STATIC_DIR : %w", err)
	}

	serveIndex := func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return fiber.ErrNotFound
		}
		setSPAHeaders(c)
		// index.html porte les references vers les bundles hashes : il doit
		// etre revalide a chaque chargement, sinon un onglet deja ouvert ne
		// verrait jamais un nouveau deploiement.
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Type("html", "utf-8")
		// Statut explicite : appele en repli du serveur de fichiers, la
		// reponse porte deja le 404 qu'il vient de poser.
		return c.Status(fiber.StatusOK).Send(index)
	}

	app.Get("/*", static.New(dir, static.Config{
		// Compression faite une fois par fichier puis mise en cache a cote de
		// l'original : le repertoire doit donc etre accessible en ecriture.
		Compress: true,
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/")
		},
		ModifyResponse: func(c fiber.Ctx) error {
			setSPAHeaders(c)
			if strings.HasPrefix(c.Path(), "/assets/") {
				// Noms hashes par Vite : le contenu d'une URL ne change
				// jamais, donc cache permanent sans revalidation.
				c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
			} else {
				c.Set(fiber.HeaderCacheControl, "no-cache")
			}
			return nil
		},
		NotFoundHandler: serveIndex,
	}), serveIndex)

	return nil
}

func setSPAHeaders(c fiber.Ctx) {
	// L'application n'a rien a faire dans un index de moteur de recherche.
	c.Set("X-Robots-Tag", "noindex, nofollow")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderXFrameOptions, "DENY")
	c.Set(fiber.HeaderReferrerPolicy, "strict-origin-when-cross-origin")
}
