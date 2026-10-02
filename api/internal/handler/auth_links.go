package handler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// AuthLinksService est le contrat des parcours publics par lien : accepter une
// invitation, reinitialiser un mot de passe oublie.
type AuthLinksService interface {
	Invitation(ctx context.Context, raw string) (usecase.InvitationView, error)
	AcceptInvitation(ctx context.Context, raw string, in usecase.AcceptInput) (usecase.Session, error)
	ForgotPassword(ctx context.Context, email string) error
	MailEnabled() bool
	PasswordResetEmail(ctx context.Context, raw string) (string, error)
	ResetPassword(ctx context.Context, raw, password string) (string, error)
}

// Bornes des parcours publics. Ils s'ouvrent sans session : ce sont les
// compteurs qui empechent de les marteler.
const (
	// Dix demandes de lien par adresse IP et par quart d'heure : assez pour
	// une agence derriere une seule adresse, trop peu pour inonder une boite.
	forgotMaxPerIP = 10
	// Vingt essais de jeton par IP : un jeton de 256 bits ne se devine pas,
	// la borne sert a couper court a un balayage.
	linkMaxPerIP = 20
	linkWindow   = 15 * time.Minute
)

// AuthLinks porte les parcours publics par lien.
type AuthLinks struct {
	svc     AuthLinksService
	cookies CookieConfig
	limiter RateLimiter
	log     *slog.Logger
}

// NewAuthLinks construit le handler.
func NewAuthLinks(svc AuthLinksService, cookies CookieConfig, limiter RateLimiter, log *slog.Logger) *AuthLinks {
	return &AuthLinks{svc: svc, cookies: cookies, limiter: limiter, log: log}
}

// Invitation sert la page d'acceptation : a qui s'adresse l'invitation, de la
// part de qui.
func (h *AuthLinks) Invitation(c fiber.Ctx) error {
	if err := h.limit(c, "link:ip:", linkMaxPerIP); err != nil {
		return err
	}

	view, err := h.svc.Invitation(c.Context(), c.Params("token"))
	if err != nil {
		return err
	}
	return c.JSON(view)
}

type acceptRequest struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Password  string `json:"password"`
}

// AcceptInvitation cree le compte et ouvre la session : l'invite arrive
// connecte.
func (h *AuthLinks) AcceptInvitation(c fiber.Ctx) error {
	if err := h.limit(c, "link:ip:", linkMaxPerIP); err != nil {
		return err
	}

	var req acceptRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	session, err := h.svc.AcceptInvitation(c.Context(), c.Params("token"), usecase.AcceptInput{
		Firstname: req.Firstname, Lastname: req.Lastname, Password: req.Password,
		UserAgent: c.Get(fiber.HeaderUserAgent), IP: clientIP(c),
	})
	if err != nil {
		return err
	}

	setSession(c, session, h.cookies)

	return c.Status(fiber.StatusCreated).JSON(sessionResponse{User: session.Profile})
}

// ForgotConfig dit a la page « mot de passe oublie » si les e-mails partent :
// sans envoi, elle oriente vers un administrateur au lieu de promettre un
// e-mail qui n'arrivera jamais.
func (h *AuthLinks) ForgotConfig(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"mail_enabled": h.svc.MailEnabled()})
}

type forgotRequest struct {
	Email string `json:"email"`
}

// Forgot envoie un lien de reinitialisation si l'adresse est celle d'un
// compte actif. 202 dans tous les cas : la reponse ne dit pas si l'adresse
// existe.
func (h *AuthLinks) Forgot(c fiber.Ctx) error {
	var req forgotRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	if !strings.Contains(req.Email, "@") {
		return domain.ErrValidation.WithDetails(map[string]any{"email": "Adresse invalide"})
	}

	if err := h.limit(c, "forgot:ip:", forgotMaxPerIP); err != nil {
		return err
	}

	if err := h.svc.ForgotPassword(c.Context(), req.Email); err != nil {
		return err
	}
	// Un objet vide et non SendStatus : Fiber mettrait « Accepted » en texte
	// dans le corps, que le client lirait comme du JSON invalide.
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{})
}

// ResetInfo sert la page de reinitialisation : pour quel compte.
func (h *AuthLinks) ResetInfo(c fiber.Ctx) error {
	if err := h.limit(c, "link:ip:", linkMaxPerIP); err != nil {
		return err
	}

	email, err := h.svc.PasswordResetEmail(c.Context(), c.Params("token"))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"email": email})
}

type resetRequest struct {
	Password string `json:"password"`
}

// Reset change le mot de passe. Toutes les sessions du compte sont fermees :
// la personne se reconnecte avec le nouveau.
func (h *AuthLinks) Reset(c fiber.Ctx) error {
	if err := h.limit(c, "link:ip:", linkMaxPerIP); err != nil {
		return err
	}

	var req resetRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	email, err := h.svc.ResetPassword(c.Context(), c.Params("token"), req.Password)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"email": email})
}

// limit applique un compteur par IP. Meme parti que la connexion : une panne
// de Redis laisse passer, elle ne bloque personne.
func (h *AuthLinks) limit(c fiber.Ctx, prefix string, limit int) error {
	allowed, retryAfter, err := h.limiter.Allow(c.Context(), prefix+c.IP(), limit, linkWindow)
	if err != nil {
		h.log.Warn("compteur de tentatives indisponible, tentative autorisee", "error", err)
		return nil
	}
	if !allowed {
		seconds := max(int(retryAfter.Seconds()), 1)
		c.Set(fiber.HeaderRetryAfter, strconv.Itoa(seconds))
		return domain.ErrRateLimited
	}
	return nil
}
