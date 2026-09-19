package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// CrmContactService est le contrat dont l'ecran « Contacts » a besoin.
type CrmContactService interface {
	List(ctx context.Context, f usecase.CrmContactFilters) (usecase.CrmContactPage, error)
	Create(ctx context.Context, in usecase.CreateContactInput) (usecase.CrmContactItem, error)
	ListOfClient(ctx context.Context, clientID uuid.UUID, search *string) ([]usecase.ContactOption, error)
	ListFree(ctx context.Context, search *string) ([]usecase.ContactRef, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.UpdateContactInput) (usecase.CrmContactItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Contacts porte les endpoints des contacts.
type Contacts struct {
	svc CrmContactService
}

// NewContacts construit le handler.
func NewContacts(svc CrmContactService) *Contacts {
	return &Contacts{svc: svc}
}

// List sert le tableau « Contacts ».
func (h *Contacts) List(c fiber.Ctx) error {
	filters := usecase.CrmContactFilters{
		Sort:     c.Query("sort", "name"),
		Dir:      c.Query("dir", "asc"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 25),
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		filters.Search = &search
	}
	if raw := strings.TrimSpace(c.Query("client_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"client_id": "Identifiant invalide"})
		}
		filters.ClientID = &id
	}

	// Seul « true » pose le filtre : un parametre incomprehensible ne doit pas
	// silencieusement reduire la liste.
	filters.OnlyFree = strings.TrimSpace(c.Query("only_free")) == "true"

	page, err := h.svc.List(c.Context(), filters)
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// createContactRequest est le corps de POST /admin/crm/contacts.
type createContactRequest struct {
	// Vide pour un contact libre, en attente d'une entreprise.
	ClientID  string `json:"client_id"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Role      string `json:"role"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Primary   bool   `json:"primary"`
}

// Create inscrit un contact.
func (h *Contacts) Create(c fiber.Ctx) error {
	var req createContactRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	var clientID *uuid.UUID
	if raw := strings.TrimSpace(req.ClientID); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{
				"client_id": "Identifiant invalide",
			})
		}

		clientID = &parsed
	}

	contact, err := h.svc.Create(c.Context(), usecase.CreateContactInput{
		ClientID:  clientID,
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
		Email:     req.Email,
		Phone:     req.Phone,
		Primary:   req.Primary,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(contact)
}

// ListOfClient sert le menu deroulant qui designe le contact principal.
func (h *Contacts) ListOfClient(c fiber.Ctx) error {
	clientID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	var search *string
	if raw := strings.TrimSpace(c.Query("search")); raw != "" {
		search = &raw
	}

	items, err := h.svc.ListOfClient(c.Context(), clientID, search)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": items})
}

// ListFree sert le menu deroulant du formulaire de creation d'un client.
func (h *Contacts) ListFree(c fiber.Ctx) error {
	var search *string
	if raw := strings.TrimSpace(c.Query("search")); raw != "" {
		search = &raw
	}

	items, err := h.svc.ListFree(c.Context(), search)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": items})
}

// updateContactRequest est le corps de PATCH /admin/crm/contacts/:id.
//
// Pas de `client_id` : rattacher quelqu'un passe par la designation du contact
// principal, qui sait tenir la cle etrangere composite dans le bon ordre.
type updateContactRequest struct {
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Role      string `json:"role"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

// Update modifie un contact.
func (h *Contacts) Update(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	var req updateContactRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	contact, err := h.svc.Update(c.Context(), id, usecase.UpdateContactInput{
		Firstname: req.Firstname,
		Lastname:  req.Lastname,
		Role:      req.Role,
		Email:     req.Email,
		Phone:     req.Phone,
	})
	if err != nil {
		return err
	}

	return c.JSON(contact)
}

// Delete efface un contact.
func (h *Contacts) Delete(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"id": "Identifiant invalide"})
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
