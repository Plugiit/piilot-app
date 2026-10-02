package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// ProjectTemplateService est le contrat de l'ecran « Modeles de projet ».
type ProjectTemplateService interface {
	List(ctx context.Context, page, pageSize int) (usecase.TemplatePage, error)
	Get(ctx context.Context, id uuid.UUID) (usecase.ProjectTemplate, error)
	Create(ctx context.Context, in usecase.TemplateInput, createdBy uuid.UUID) (usecase.ProjectTemplate, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.TemplateInput) (usecase.ProjectTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProjectTemplates porte les modeles de projet.
type ProjectTemplates struct {
	svc ProjectTemplateService
}

// NewProjectTemplates construit le handler.
func NewProjectTemplates(svc ProjectTemplateService) *ProjectTemplates {
	return &ProjectTemplates{svc: svc}
}

// templateBody est un modele entier, tel que l'editeur l'enregistre.
type templateBody struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	ServiceIDs  []string                    `json:"service_ids"`
	Milestones  []usecase.TemplateMilestone `json:"milestones"`
	Tasks       []usecase.TemplateTask      `json:"tasks"`
}

func (b templateBody) input() (usecase.TemplateInput, error) {
	services, err := parseUUIDs(b.ServiceIDs, "service_ids")
	if err != nil {
		return usecase.TemplateInput{}, err
	}

	return usecase.TemplateInput{
		Name:        b.Name,
		Description: b.Description,
		ServiceIDs:  services,
		Milestones:  b.Milestones,
		Tasks:       b.Tasks,
	}, nil
}

// List sert la liste des modeles.
func (h *ProjectTemplates) List(c fiber.Ctx) error {
	page, err := h.svc.List(c.Context(), queryInt(c, "page", 1), queryInt(c, "page_size", 25))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Get sert un modele et son contenu.
func (h *ProjectTemplates) Get(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	template, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(template)
}

// Create enregistre un nouveau modele.
func (h *ProjectTemplates) Create(c fiber.Ctx) error {
	var body templateBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in, err := body.input()
	if err != nil {
		return err
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	template, err := h.svc.Create(c.Context(), in, actor)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(template)
}

// Update remplace un modele et son contenu.
func (h *ProjectTemplates) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body templateBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in, err := body.input()
	if err != nil {
		return err
	}

	template, err := h.svc.Update(c.Context(), id, in)
	if err != nil {
		return err
	}

	return c.JSON(template)
}

// Delete supprime un modele.
func (h *ProjectTemplates) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
