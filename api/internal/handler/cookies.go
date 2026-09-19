package handler

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// RefreshCookieName porte le jeton de rafraichissement, separe du jeton
// d'acces : les deux n'ont ni la meme duree ni la meme portee.
const RefreshCookieName = "plugiit_refresh"

// refreshCookiePath restreint l'envoi du cookie de rafraichissement au groupe
// /auth. Il ne part donc pas sur les requetes metier, ou il n'aurait rien a
// faire et ou chaque envoi serait une occasion de fuite supplementaire.
//
// La portee couvre /login, /logout et /refresh — et non le seul /refresh :
// sans cela, /logout ne recevrait pas le cookie et ne pourrait pas revoquer la
// session cote serveur.
const refreshCookiePath = "/api/v1/auth"

// CookieConfig regle les attributs de securite des cookies de session.
type CookieConfig struct {
	// Domain vide lie le cookie au seul hote qui l'a pose : c'est le reglage le
	// plus etroit, a ne desserrer que si un alias de sous-domaine apparait.
	Domain string
	// Secure interdit l'envoi hors HTTPS. Faux en developpement local, ou
	// l'API est servie en clair sur localhost.
	Secure bool
}

// setSession pose les deux cookies de session.
//
// httpOnly sur les deux : le JavaScript de l'application n'y accede jamais,
// donc une faille XSS ne permet pas de les exfiltrer. SameSite=Lax bloque
// l'envoi depuis un site tiers, ce qui couvre le CSRF sur les requetes
// non-GET sans machinerie de jeton dedie.
func setSession(c fiber.Ctx, s usecase.Session, cfg CookieConfig) {
	c.Cookie(&fiber.Cookie{
		Name:     middleware.AccessCookieName,
		Value:    s.AccessToken,
		Path:     "/",
		Domain:   cfg.Domain,
		Expires:  s.AccessExpiresAt,
		Secure:   cfg.Secure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	c.Cookie(&fiber.Cookie{
		Name:     RefreshCookieName,
		Value:    s.RefreshToken,
		Path:     refreshCookiePath,
		Domain:   cfg.Domain,
		Expires:  s.RefreshExpiresAt,
		Secure:   cfg.Secure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}

// clearSession efface les deux cookies.
//
// Les attributs Path et Domain doivent etre identiques a ceux de la pose : un
// navigateur ne remplace pas un cookie dont la portee differe, il en cree un
// second — et l'ancien continuerait de partir.
func clearSession(c fiber.Ctx, cfg CookieConfig) {
	expired := time.Now().Add(-time.Hour)

	c.Cookie(&fiber.Cookie{
		Name:     middleware.AccessCookieName,
		Value:    "",
		Path:     "/",
		Domain:   cfg.Domain,
		Expires:  expired,
		MaxAge:   -1,
		Secure:   cfg.Secure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	c.Cookie(&fiber.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		Domain:   cfg.Domain,
		Expires:  expired,
		MaxAge:   -1,
		Secure:   cfg.Secure,
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
}
