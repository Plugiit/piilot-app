// Package middleware regroupe les middlewares HTTP transverses.
package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/piilot-app/api/internal/domain"
)

// errorResponse est la forme unique des erreurs de l'API. Le front s'appuie
// dessus pour tous les endpoints, ce qui evite un traitement d'erreur par vue.
type errorResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// ErrorHandler traduit les erreurs remontees par les handlers en JSON.
//
// Les *domain.Error portent leur propre statut et un code stable. Tout le reste
// devient un 500 opaque : la cause part dans les logs, jamais dans la reponse,
// pour ne pas fuiter de detail d'implementation.
func ErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		if domainErr, ok := domain.AsError(err); ok {
			if domainErr.Status >= 500 {
				log.Error("erreur serveur",
					"code", domainErr.Code,
					"path", c.Path(),
					"method", c.Method(),
					"request_id", requestID(c),
					"error", err,
				)
			}

			return c.Status(domainErr.Status).JSON(errorResponse{
				Code:    domainErr.Code,
				Message: domainErr.Message,
				Details: details(domainErr.Details),
			})
		}

		// Erreurs levees par Fiber lui-meme (404 de routage, body trop gros...).
		var fiberErr *fiber.Error
		if ok := asFiberError(err, &fiberErr); ok {
			return c.Status(fiberErr.Code).JSON(errorResponse{
				Code:    fiberCode(fiberErr.Code),
				Message: fiberErr.Message,
				Details: map[string]any{},
			})
		}

		log.Error("erreur non geree",
			"path", c.Path(),
			"method", c.Method(),
			"request_id", requestID(c),
			"error", err,
		)

		return c.Status(domain.ErrInternal.Status).JSON(errorResponse{
			Code:    domain.ErrInternal.Code,
			Message: domain.ErrInternal.Message,
			Details: map[string]any{},
		})
	}
}

// details garantit un objet JSON plutot qu'un null, pour que le front puisse
// toujours indexer le champ sans garde.
func details(d map[string]any) map[string]any {
	if d == nil {
		return map[string]any{}
	}
	return d
}

func fiberCode(status int) string {
	switch status {
	case fiber.StatusNotFound:
		return domain.ErrNotFound.Code
	case fiber.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case fiber.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	default:
		return "HTTP_ERROR"
	}
}
