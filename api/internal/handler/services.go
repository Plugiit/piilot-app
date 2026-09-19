package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ServiceService est le contrat dont le referentiel des prestations a besoin.
type ServiceService interface {
	List(ctx context.Context, search *string, page, pageSize int) (usecase.ServicePage, error)
	Create(ctx context.Context, in usecase.ServiceInput) (usecase.Service, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.ServiceInput) (usecase.Service, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Services porte les endpoints du referentiel.
type Services struct {
	svc ServiceService
}

// NewServices construit le handler.
func NewServices(svc ServiceService) *Services {
	return &Services{svc: svc}
}

// serviceBody est ce que le formulaire envoie, a la creation comme a la
// modification : les deux montrent les memes champs.
type serviceBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func (b serviceBody) input() usecase.ServiceInput {
	return usecase.ServiceInput{Name: b.Name, Description: b.Description, Color: b.Color}
}

// List sert le tableau du referentiel.
func (h *Services) List(c fiber.Ctx) error {
	var search *string
	if raw := strings.TrimSpace(c.Query("search")); raw != "" {
		search = &raw
	}

	page, err := h.svc.List(
		c.Context(),
		search,
		queryInt(c, "page", 1),
		queryInt(c, "page_size", 25),
	)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Create ajoute une prestation.
func (h *Services) Create(c fiber.Ctx) error {
	var body serviceBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	service, err := h.svc.Create(c.Context(), body.input())
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(service)
}

// Update modifie une prestation.
func (h *Services) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body serviceBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	service, err := h.svc.Update(c.Context(), id, body.input())
	if err != nil {
		return err
	}

	return c.JSON(service)
}

// Delete retire une prestation.
func (h *Services) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
