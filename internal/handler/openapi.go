package handler

import (
	"github.com/gofiber/fiber/v3"

	"github.com/plugiit/plugiit-api-go/openapi"
)

// openapiSpec sert le contrat HTTP de l'API.
//
// Servi par l'API elle-meme plutot que copie dans le depot du front : les deux
// depots ont des historiques independants, et une spec dupliquee se
// desynchronise a la premiere modification faite d'un seul cote.
func openapiSpec(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	return c.Send(openapi.Spec)
}
