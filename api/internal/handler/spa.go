package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/plugiit/piilot-app/api/internal/spaces"
)

// RegisterSPA sert le build du front depuis dir, apres toutes les routes de
// l'API : une route d'API l'emporte toujours, le front ne recoit que le reste.
//
// Le routage est cote client : une URL qui ne designe aucun fichier rend
// index.html, et TanStack Router affiche l'ecran. Sauf sous /api/ — une route
// d'API inconnue doit rester un 404 JSON, pas une page HTML qu'un appelant
// essaierait de decoder.
//
// En mode multi-domaines, une page ouverte sur le mauvais domaine est renvoyee
// vers celui de son espace, et index.html recoit les quatre domaines : le front
// en a besoin pour faire de meme quand on navigue sans recharger la page.
//
// Si l'instance mesure son audience, index.html recoit aussi l'adresse du
// script Umami : le front le charge lui-meme, apres avoir pose son filtre.
func RegisterSPA(app *fiber.App, dir string, sp spaces.Spaces, tr SPATracker) error {
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return fmt.Errorf("front introuvable dans STATIC_DIR : %w", err)
	}
	index = injectSpaces(index, sp)
	index = injectTracker(index, tr)

	serveIndex := func(c fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return fiber.ErrNotFound
		}
		if target := sp.Redirect(c.Host(), c.Path(), string(c.Request().URI().QueryString())); target != "" {
			return c.Redirect().Status(fiber.StatusFound).To(target)
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
		// index.html passe toujours par serveIndex, y compris a la racine :
		// c'est lui qui porte les domaines des espaces.
		Next: func(c fiber.Ctx) bool {
			return strings.HasPrefix(c.Path(), "/api/") || c.Path() == "/" || c.Path() == "/index.html"
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

// injectSpaces ajoute a index.html une balise meta portant les domaines des
// espaces. Une meta plutot qu'un script en ligne : la politique de securite du
// contenu n'a pas a autoriser de code inline pour une simple donnee.
func injectSpaces(index []byte, sp spaces.Spaces) []byte {
	if !sp.Enabled() {
		return index
	}

	payload, _ := json.Marshal(map[string]string{
		"auth": sp.AuthURL, "team": sp.TeamURL, "admin": sp.AdminURL, "client": sp.ClientURL,
	})
	return injectMeta(index, "piilot-spaces", payload)
}

// SPATracker est le script Umami qui mesure l'audience de l'instance. Vide,
// aucune page n'en charge.
type SPATracker struct {
	ScriptURL string
	WebsiteID string
}

// injectTracker ajoute a index.html l'adresse du script Umami, dans une meta
// et non dans une balise script : charge directement, le script enverrait les
// adresses telles quelles, jetons de reinitialisation et d'invitation compris.
// Le front le charge apres avoir pose le filtre qui les retire ; si le front
// ne demarre pas, rien ne part.
func injectTracker(index []byte, tr SPATracker) []byte {
	if tr.ScriptURL == "" {
		return index
	}

	payload, _ := json.Marshal(map[string]string{"src": tr.ScriptURL, "websiteId": tr.WebsiteID})
	return injectMeta(index, "piilot-tracker", payload)
}

func injectMeta(index []byte, name string, payload []byte) []byte {
	meta := []byte(`<meta name="` + name + `" content="` + html.EscapeString(string(payload)) + `">`)
	return bytes.Replace(index, []byte("</head>"), append(meta, []byte("</head>")...), 1)
}
