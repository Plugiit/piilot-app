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

// TicketService est le contrat dont l'ecran « Tickets » a besoin.
type TicketService interface {
	ListAssignedTo(ctx context.Context, userID uuid.UUID, f usecase.TicketFilters, page, pageSize int) (usecase.TicketPage, error)
	Create(ctx context.Context, in usecase.CreateTicketInput) (usecase.TicketItem, error)
	BoardAssignedTo(ctx context.Context, userID uuid.UUID, f usecase.TicketFilters) (usecase.TicketBoard, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, f usecase.TicketFilters, page, pageSize int) (usecase.TicketPage, error)
	BoardByProject(ctx context.Context, projectID uuid.UUID, f usecase.TicketFilters) (usecase.TicketBoard, error)
	Get(ctx context.Context, id uuid.UUID) (usecase.TicketDetail, error)
	PostMessage(ctx context.Context, ticketID uuid.UUID, in usecase.PostMessageInput) (usecase.TicketDetail, error)
	Rename(ctx context.Context, ticketID uuid.UUID, subject string, actorID *uuid.UUID) (usecase.TicketDetail, error)
}

// Tickets porte les endpoints des tickets.
type Tickets struct {
	svc TicketService
}

// NewTickets construit le handler.
func NewTickets(svc TicketService) *Tickets {
	return &Tickets{svc: svc}
}

// ticketFiltersFrom lit la barre d'outils depuis l'adresse.
//
// Partage par le tableau et par les deux kanbans : ils montrent les memes
// tickets, et changer d'onglet ne doit pas remettre les filtres a zero.
//
// Les valeurs ne sont pas validees contre les nomenclatures : un statut inconnu
// ne correspond a aucune ligne et rend une liste vide, ce qui est le resultat
// juste. Refuser en 422 ferait echouer un ecran sur un parametre d'adresse
// bricole, pour le meme resultat visible.
func ticketFiltersFrom(c fiber.Ctx) (usecase.TicketFilters, error) {
	filters := usecase.TicketFilters{}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filters.Search = &search
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filters.Status = &status
	}
	if tracker := strings.TrimSpace(c.Query("tracker")); tracker != "" {
		filters.Tracker = &tracker
	}
	if priority := strings.TrimSpace(c.Query("priority")); priority != "" {
		filters.Priority = &priority
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

// Mine sert le tableau « Tickets » : ceux qui reviennent a la personne
// connectee.
//
// L'identifiant est lu dans la session et n'est pas un parametre : c'est ce qui
// empeche de demander les tickets de quelqu'un d'autre en changeant l'adresse.
func (h *Tickets) Mine(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	filters, err := ticketFiltersFrom(c)
	if err != nil {
		return err
	}

	page, err := h.svc.ListAssignedTo(
		c.Context(),
		userID,
		filters,
		queryInt(c, "page", 1),
		queryInt(c, "page_size", 25),
	)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// MineBoard sert les deux vues kanban : les memes cartes, que le front
// regroupe par projet ou par statut.
//
// Un seul endpoint pour les deux : le regroupement est une facon de lire, pas
// un jeu de donnees different, et deux routes auraient servi deux fois la meme
// reponse.
func (h *Tickets) MineBoard(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	filters, err := ticketFiltersFrom(c)
	if err != nil {
		return err
	}

	board, err := h.svc.BoardAssignedTo(c.Context(), userID, filters)
	if err != nil {
		return err
	}

	return c.JSON(board)
}

// ProjectList sert le tableau de l'onglet « Tickets » d'un projet.
//
// Le projet vient de l'adresse et non des filtres : c'est la fiche qu'on lit.
// Un `project_id` en parametre de recherche y serait sans effet.
func (h *Tickets) ProjectList(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	filters, err := ticketFiltersFrom(c)
	if err != nil {
		return err
	}

	page, err := h.svc.ListByProject(
		c.Context(),
		projectID,
		filters,
		queryInt(c, "page", 1),
		queryInt(c, "page_size", 25),
	)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// ProjectBoard sert le kanban du meme onglet : les memes cartes, que le front
// repartit par statut.
func (h *Tickets) ProjectBoard(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	filters, err := ticketFiltersFrom(c)
	if err != nil {
		return err
	}

	board, err := h.svc.BoardByProject(c.Context(), projectID, filters)
	if err != nil {
		return err
	}

	return c.JSON(board)
}

// createTicketBody est ce que le formulaire de depot envoie.
//
// Pas de `status` : un ticket nait dans le backlog. Pas de `numero` non plus,
// c'est la base qui l'attribue.
type createTicketBody struct {
	ProjectID   string  `json:"project_id"`
	Subject     string  `json:"subject"`
	Description string  `json:"description"`
	Tracker     string  `json:"tracker"`
	Priority    string  `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
}

// Create depose un ticket.
func (h *Tickets) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	var body createTicketBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"body": "Corps de requête illisible",
		})
	}

	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"project_id": "Identifiant de projet invalide",
		})
	}

	// L'assignation est facultative : un ticket depose sans destinataire reste
	// a prendre, ce qui est un etat normal et non une saisie incomplete.
	var assignee *uuid.UUID

	if body.AssigneeID != nil && *body.AssigneeID != "" {
		parsed, err := uuid.Parse(*body.AssigneeID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"assignee_id": "Identifiant de compte invalide",
			})
		}

		assignee = &parsed
	}

	ticket, err := h.svc.Create(c.Context(), usecase.CreateTicketInput{
		ProjectID:   projectID,
		Subject:     body.Subject,
		Description: body.Description,
		Tracker:     body.Tracker,
		Priority:    body.Priority,
		AssigneeID:  assignee,
		CreatedBy:   &userID,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(ticket)
}

// ticketIDFrom lit l'identifiant du ticket dans l'adresse.
func ticketIDFrom(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, domain.ErrValidation.WithDetails(map[string]any{
			"id": "Identifiant de ticket invalide",
		})
	}

	return id, nil
}

// Get sert la fiche d'un ticket : son en-tete, ses coordonnees et son registre.
//
// Un seul appel remplit l'ecran, comme la fiche d'un client : le registre et
// l'en-tete arrivent ensemble plutot qu'en deux requetes en cascade.
func (h *Tickets) Get(c fiber.Ctx) error {
	id, err := ticketIDFrom(c)
	if err != nil {
		return err
	}

	ticket, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(ticket)
}

// postMessageBody est ce que le redacteur envoie.
type postMessageBody struct {
	Body       string `json:"body"`
	IsInternal bool   `json:"is_internal"`
	// Nuls ou absents pour ne rien changer.
	Status   *string `json:"status"`
	Priority *string `json:"priority"`
	// L'assignation a son propre drapeau : `assignee_id` nul avec
	// `change_assignee` vrai remet le ticket a prendre, ce qu'un pointeur seul
	// ne saurait pas distinguer de « ne touche pas a l'assignation ».
	ChangeAssignee bool    `json:"change_assignee"`
	AssigneeID     *string `json:"assignee_id"`
}

// PostMessage inscrit une entree au registre, et deplace le ticket si l'entree
// le demande.
func (h *Tickets) PostMessage(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	id, err := ticketIDFrom(c)
	if err != nil {
		return err
	}

	var body postMessageBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"body": "Corps de requête illisible",
		})
	}

	var assignee *uuid.UUID

	if body.ChangeAssignee && body.AssigneeID != nil && strings.TrimSpace(*body.AssigneeID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*body.AssigneeID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"assignee_id": "Identifiant de compte invalide",
			})
		}

		assignee = &parsed
	}

	ticket, err := h.svc.PostMessage(c.Context(), id, usecase.PostMessageInput{
		Body:           body.Body,
		IsInternal:     body.IsInternal,
		AuthorID:       &userID,
		NewStatus:      optionalText(body.Status),
		NewPriority:    optionalText(body.Priority),
		ChangeAssignee: body.ChangeAssignee,
		NewAssigneeID:  assignee,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(ticket)
}

// optionalText rend nil pour un champ absent ou vide : les deux veulent dire
// « ne change pas », et l'ecran envoie l'un ou l'autre selon le controle.
func optionalText(raw *string) *string {
	if raw == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

// renameBody est ce que le titre modifiable envoie.
type renameBody struct {
	Subject string `json:"subject"`
}

// Rename change le sujet d'un ticket.
func (h *Tickets) Rename(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	id, err := ticketIDFrom(c)
	if err != nil {
		return err
	}

	var body renameBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{
			"body": "Corps de requête illisible",
		})
	}

	ticket, err := h.svc.Rename(c.Context(), id, body.Subject, &userID)
	if err != nil {
		return err
	}

	return c.JSON(ticket)
}
