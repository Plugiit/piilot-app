package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// DeliverableService est le contrat dont l'ecran « Livrables » a besoin.
type DeliverableService interface {
	List(ctx context.Context, f usecase.DeliverableFilters, page, pageSize int) (usecase.DeliverablePage, error)
	Create(ctx context.Context, in usecase.CreateDeliverableInput) (usecase.DeliverableItem, error)
	Submit(ctx context.Context, deliverableID uuid.UUID, url string, submittedBy *uuid.UUID) (usecase.DeliverableItem, error)
	Decide(ctx context.Context, deliverableID uuid.UUID, decision, feedback string, decidedBy *uuid.UUID) (usecase.DeliverableItem, error)
	Versions(ctx context.Context, deliverableID uuid.UUID) ([]usecase.DeliverableEntry, error)
}

// Deliverables porte les endpoints des livrables.
type Deliverables struct {
	svc DeliverableService
}

// NewDeliverables construit le handler.
func NewDeliverables(svc DeliverableService) *Deliverables {
	return &Deliverables{svc: svc}
}

// deliverableFiltersFrom lit la barre d'outils depuis l'adresse.
//
// Comme pour les tickets, les valeurs ne sont pas validees contre la
// nomenclature : un statut inconnu ne correspond a aucune ligne et rend une
// liste vide, ce qui est le resultat juste.
func deliverableFiltersFrom(c fiber.Ctx) (usecase.DeliverableFilters, error) {
	filters := usecase.DeliverableFilters{}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filters.Search = &search
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filters.Status = &status
	}
	if raw := strings.TrimSpace(c.Query("project_id")); raw != "" {
		projectID, err := uuid.Parse(raw)
		if err != nil {
			return filters, domain.ErrValidation.WithDetails(map[string]any{
				"project_id": "Identifiant de projet invalide",
			})
		}

		filters.ProjectID = &projectID
	}

	return filters, nil
}

// List sert l'ecran « Livrables » du module, qui traverse les projets.
func (h *Deliverables) List(c fiber.Ctx) error {
	filters, err := deliverableFiltersFrom(c)
	if err != nil {
		return err
	}

	page, err := h.svc.List(
		c.Context(),
		filters,
		queryInt(c, "page", 1),
		queryInt(c, "page_size", 25),
	)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// createDeliverableBody est ce que le formulaire de depot envoie.
//
// Pas de statut : un livrable nait avec sa premiere version, donc en attente.
type createDeliverableBody struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// Create depose un livrable et sa premiere version sur un projet.
func (h *Deliverables) Create(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body createDeliverableBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	actor := actorFrom(c)

	item, err := h.svc.Create(c.Context(), usecase.CreateDeliverableInput{
		ProjectID:   projectID,
		Title:       body.Title,
		Description: body.Description,
		URL:         body.URL,
		CreatedBy:   actor,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

// submitVersionBody est ce que le depot d'une nouvelle version envoie.
type submitVersionBody struct {
	URL string `json:"url"`
}

// Submit ajoute une version a un livrable : la v2 apres des retours.
func (h *Deliverables) Submit(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body submitVersionBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	item, err := h.svc.Submit(c.Context(), id, body.URL, actorFrom(c))
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

// decideBody est la reponse rendue sur la version courante.
type decideBody struct {
	Decision string `json:"decision"`
	Feedback string `json:"feedback"`
}

// Decide enregistre la reponse du client.
//
// La route exige `deliverables.validate`, que portent le role client et le role
// team : un client tranche pour lui-meme, l'agence enregistre une reponse recue
// au telephone. C'est le role du compte qui dit lequel des deux.
func (h *Deliverables) Decide(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body decideBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	item, err := h.svc.Decide(c.Context(), id, body.Decision, body.Feedback, actorFrom(c))
	if err != nil {
		return err
	}

	return c.JSON(item)
}

// Versions sert le fil d'un livrable : toutes ses versions et leurs decisions.
func (h *Deliverables) Versions(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	entries, err := h.svc.Versions(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": entries})
}

// actorFrom rend le compte appelant, ou nil hors session.
//
// Nul plutot qu'une erreur : les colonnes d'auteur acceptent l'absence, et un
// livrable sans auteur vaut mieux qu'un depot refuse.
func actorFrom(c fiber.Ctx) *uuid.UUID {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return nil
	}

	return &userID
}
