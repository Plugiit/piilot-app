package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// GitService est ce que le handler attend de l'integration Git.
type GitService interface {
	Secret(ctx context.Context) (string, error)
	Settings(ctx context.Context) (usecase.GitSettings, error)
	Rotate(ctx context.Context) (usecase.GitSettings, error)
	HandleGitHub(ctx context.Context, event string, body []byte) (usecase.Outcome, error)
	HandleGitLab(ctx context.Context, body []byte) (usecase.Outcome, error)
}

// Git recoit les webhooks de GitHub et GitLab, et sert leur reglage.
type Git struct {
	svc GitService
}

func NewGit(svc GitService) *Git { return &Git{svc: svc} }

// Webhook recoit un evenement. Pas de session : c'est la signature qui
// prouve l'appelant — HMAC du corps chez GitHub, jeton en clair chez GitLab.
// Un secret faux repond 401 sans rien dire d'autre.
func (h *Git) Webhook(c fiber.Ctx) error {
	secret, err := h.svc.Secret(c.Context())
	if err != nil {
		return err
	}
	body := c.Body()

	if event := c.Get("X-GitHub-Event"); event != "" {
		if !githubSignatureValid(secret, c.Get("X-Hub-Signature-256"), body) {
			return domain.ErrUnauthorized
		}
		out, err := h.svc.HandleGitHub(c.Context(), event, body)
		if err != nil {
			return err
		}
		return c.Status(fiber.StatusAccepted).JSON(out)
	}

	if c.Get("X-Gitlab-Event") != "" {
		if subtle.ConstantTimeCompare([]byte(c.Get("X-Gitlab-Token")), []byte(secret)) != 1 {
			return domain.ErrUnauthorized
		}
		out, err := h.svc.HandleGitLab(c.Context(), body)
		if err != nil {
			return err
		}
		return c.Status(fiber.StatusAccepted).JSON(out)
	}

	return domain.ErrValidation.WithMessage("Ni GitHub ni GitLab : en-tête X-GitHub-Event ou X-Gitlab-Event attendu")
}

// githubSignatureValid verifie « sha256=<hex> » contre le HMAC du corps.
func githubSignatureValid(secret, header string, body []byte) bool {
	given, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(given), []byte(want)) == 1
}

// Settings rend l'adresse du webhook et son secret.
func (h *Git) Settings(c fiber.Ctx) error {
	out, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Rotate tire un nouveau secret.
func (h *Git) Rotate(c fiber.Ctx) error {
	out, err := h.svc.Rotate(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}
