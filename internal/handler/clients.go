package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// CrmClientService est le contrat dont l'ecran « Clients » a besoin. Declare
// chez le consommateur, comme les autres : le handler reste testable sans
// Postgres.
type CrmClientService interface {
	List(ctx context.Context, f usecase.CrmClientFilters) (usecase.CrmClientPage, error)
	Create(ctx context.Context, in usecase.CreateClientInput) (usecase.CrmClientItem, error)
	SetPrimaryContact(ctx context.Context, clientID uuid.UUID, contactID *uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (usecase.CrmClientDetail, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.UpdateClientInput) (usecase.CrmClientItem, error)
	MoveStatus(ctx context.Context, id uuid.UUID, status string) (usecase.CrmClientItem, error)
	Board(ctx context.Context, f usecase.CrmClientFilters) (usecase.CrmClientBoard, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Clients porte les endpoints du module CRM.
type Clients struct {
	svc CrmClientService
}

// NewClients construit le handler.
func NewClients(svc CrmClientService) *Clients {
	return &Clients{svc: svc}
}

// List sert le tableau « Clients ».
func (h *Clients) List(c fiber.Ctx) error {
	filters := usecase.CrmClientFilters{
		Sort:     c.Query("sort", "name"),
		Dir:      c.Query("dir", "asc"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 25),
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filters.Search = &search
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filters.Status = &status
	}
	if raw := strings.TrimSpace(c.Query("manager_id")); raw != "" {
		managerID, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"manager_id": "Identifiant invalide"})
		}
		filters.ManagerID = &managerID
	}

	// Le filtre n'est pose que sur l'une des deux valeurs attendues : absent,
	// vide ou mal ecrit, il n'existe pas et la liste ne se reduit pas. Un
	// parametre incomprehensible ne doit pas silencieusement filtrer.
	switch strings.TrimSpace(c.Query("has_portal")) {
	case "true":
		yes := true
		filters.HasPortal = &yes
	case "false":
		no := false
		filters.HasPortal = &no
	}

	page, err := h.svc.List(c.Context(), filters)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// createClientRequest est le corps de POST /admin/crm/clients.
//
// `contact_id` designe un contact libre a adopter. Facultatif : on inscrit
// souvent une entreprise avant de savoir a qui l'on parlera.
type createClientRequest struct {
	Name      string  `json:"name"`
	ContactID *string `json:"contact_id"`
}

// Create inscrit un client.
func (h *Clients) Create(c fiber.Ctx) error {
	var req createClientRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in := usecase.CreateClientInput{Name: req.Name}

	if req.ContactID != nil && strings.TrimSpace(*req.ContactID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.ContactID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"contact_id": "Identifiant invalide",
			})
		}

		in.ContactID = &id
	}

	client, err := h.svc.Create(c.Context(), in)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(client)
}

// setPrimaryContactRequest est le corps de PUT
// /admin/crm/clients/:id/primary-contact.
//
// `contact_id` nul retire la designation : un client peut cesser d'avoir un
// interlocuteur principal sans qu'on supprime personne.
type setPrimaryContactRequest struct {
	ContactID *string `json:"contact_id"`
}

// SetPrimaryContact designe l'interlocuteur principal d'un client.
func (h *Clients) SetPrimaryContact(c fiber.Ctx) error {
	clientID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	var req setPrimaryContactRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	var contactID *uuid.UUID
	if req.ContactID != nil && strings.TrimSpace(*req.ContactID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.ContactID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"contact_id": "Identifiant invalide",
			})
		}

		contactID = &parsed
	}

	if err := h.svc.SetPrimaryContact(c.Context(), clientID, contactID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Get sert la fiche d'un client.
func (h *Clients) Get(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	detail, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(detail)
}

// updateClientRequest est le corps de PATCH /admin/crm/clients/:id.
//
// Tous les champs voyagent ensemble : la requete les ecrit tous, et n'en
// envoyer qu'une partie effacerait le reste.
type updateClientRequest struct {
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	AccountManagerID *string `json:"account_manager_id"`
	Website          string  `json:"website"`
	Phone            string  `json:"phone"`
	Address          string  `json:"address"`
	PostalCode       string  `json:"postal_code"`
	City             string  `json:"city"`
	Country          string  `json:"country"`
	Siret            string  `json:"siret"`
	VatNumber        string  `json:"vat_number"`
}

// Update renomme un client.
func (h *Clients) Update(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	var req updateClientRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in := usecase.UpdateClientInput{
		Name:       req.Name,
		Status:     req.Status,
		Website:    req.Website,
		Phone:      req.Phone,
		Address:    req.Address,
		PostalCode: req.PostalCode,
		City:       req.City,
		Country:    req.Country,
		Siret:      req.Siret,
		VatNumber:  req.VatNumber,
	}

	if req.AccountManagerID != nil && strings.TrimSpace(*req.AccountManagerID) != "" {
		managerID, err := uuid.Parse(strings.TrimSpace(*req.AccountManagerID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"account_manager_id": "Identifiant invalide",
			})
		}

		in.AccountManagerID = &managerID
	}

	client, err := h.svc.Update(c.Context(), id, in)
	if err != nil {
		return err
	}

	return c.JSON(client)
}

// Delete efface un client.
func (h *Clients) Delete(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// moveClientRequest est le corps de PUT /admin/crm/clients/:id/status.
type moveClientRequest struct {
	Status string `json:"status"`
}

// MoveStatus deplace une carte du kanban.
func (h *Clients) MoveStatus(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	var req moveClientRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	client, err := h.svc.MoveStatus(c.Context(), id, req.Status)
	if err != nil {
		return err
	}

	return c.JSON(client)
}

// Board sert le kanban commercial.
func (h *Clients) Board(c fiber.Ctx) error {
	filters := usecase.CrmClientFilters{}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filters.Search = &search
	}
	if raw := strings.TrimSpace(c.Query("manager_id")); raw != "" {
		managerID, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"manager_id": "Identifiant invalide"})
		}
		filters.ManagerID = &managerID
	}

	board, err := h.svc.Board(c.Context(), filters)
	if err != nil {
		return err
	}

	return c.JSON(board)
}
