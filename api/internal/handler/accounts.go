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

// AccountService est le contrat de la gestion des comptes et des roles.
type AccountService interface {
	List(ctx context.Context, f usecase.AccountFilters) (usecase.AccountPage, error)
	Invitations(ctx context.Context) (usecase.InvitationList, error)
	Invite(ctx context.Context, in usecase.InviteInput) (usecase.InviteResult, error)
	ResendInvitation(ctx context.Context, id uuid.UUID) (usecase.SentLink, error)
	RevokeInvitation(ctx context.Context, id uuid.UUID) error
	SetRole(ctx context.Context, actor, id uuid.UUID, role string) error
	Disable(ctx context.Context, actor, id uuid.UUID) error
	Enable(ctx context.Context, actor, id uuid.UUID) error
	PasswordResetLink(ctx context.Context, actor, id uuid.UUID) (usecase.SentLink, error)
	Roles(ctx context.Context) (usecase.RoleMatrix, error)
	SetRolePermissions(ctx context.Context, role string, permissions []string) (usecase.RoleMatrix, error)
}

// Accounts porte l'ecran « Comptes et roles ».
type Accounts struct {
	svc AccountService
}

// NewAccounts construit le handler.
func NewAccounts(svc AccountService) *Accounts {
	return &Accounts{svc: svc}
}

// List sert l'onglet « Comptes ».
func (h *Accounts) List(c fiber.Ctx) error {
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	f := usecase.AccountFilters{
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 25),
		Viewer:   actor,
	}
	if v := strings.TrimSpace(c.Query("search")); v != "" {
		f.Search = &v
	}
	if v := strings.TrimSpace(c.Query("role")); v != "" {
		f.Role = &v
	}
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		if v != "active" && v != "disabled" {
			return domain.ErrValidation.WithDetails(map[string]any{"status": "Valeur attendue : active ou disabled"})
		}
		f.Status = &v
	}

	page, err := h.svc.List(c.Context(), f)
	if err != nil {
		return err
	}
	return c.JSON(page)
}

// Invitations sert l'onglet « Invitations ».
func (h *Accounts) Invitations(c fiber.Ctx) error {
	list, err := h.svc.Invitations(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(list)
}

type inviteRequest struct {
	Email     string  `json:"email"`
	Firstname string  `json:"firstname"`
	Lastname  string  `json:"lastname"`
	Role      string  `json:"role"`
	ClientID  *string `json:"client_id"`
}

// Invite envoie une invitation.
func (h *Accounts) Invite(c fiber.Ctx) error {
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	var req inviteRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in := usecase.InviteInput{
		Email: req.Email, Firstname: req.Firstname, Lastname: req.Lastname, Role: req.Role, InvitedBy: actor,
	}
	if req.ClientID != nil && *req.ClientID != "" {
		id, err := uuid.Parse(*req.ClientID)
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"client_id": "Identifiant invalide"})
		}
		in.ClientID = &id
	}

	result, err := h.svc.Invite(c.Context(), in)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// ResendInvitation renvoie une invitation avec un nouveau lien.
func (h *Accounts) ResendInvitation(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	link, err := h.svc.ResendInvitation(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(link)
}

// RevokeInvitation annule une invitation.
func (h *Accounts) RevokeInvitation(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.RevokeInvitation(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type setRoleRequest struct {
	Role string `json:"role"`
}

// SetRole change le role d'un compte.
func (h *Accounts) SetRole(c fiber.Ctx) error {
	actor, id, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}

	var req setRoleRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	if err := h.svc.SetRole(c.Context(), actor, id, req.Role); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Disable desactive un compte.
func (h *Accounts) Disable(c fiber.Ctx) error {
	actor, id, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}
	if err := h.svc.Disable(c.Context(), actor, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Enable reactive un compte.
func (h *Accounts) Enable(c fiber.Ctx) error {
	actor, id, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}
	if err := h.svc.Enable(c.Context(), actor, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PasswordReset cree un lien de reinitialisation pour un compte.
func (h *Accounts) PasswordReset(c fiber.Ctx) error {
	actor, id, err := h.actorAndTarget(c)
	if err != nil {
		return err
	}

	link, err := h.svc.PasswordResetLink(c.Context(), actor, id)
	if err != nil {
		return err
	}
	return c.JSON(link)
}

// Roles sert l'onglet « Roles ».
func (h *Accounts) Roles(c fiber.Ctx) error {
	matrix, err := h.svc.Roles(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(matrix)
}

type rolePermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

// SetRolePermissions remplace les permissions d'un role.
func (h *Accounts) SetRolePermissions(c fiber.Ctx) error {
	var req rolePermissionsRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	matrix, err := h.svc.SetRolePermissions(c.Context(), c.Params("code"), req.Permissions)
	if err != nil {
		return err
	}
	return c.JSON(matrix)
}

func (h *Accounts) actorAndTarget(c fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return uuid.Nil, uuid.Nil, domain.ErrUnauthorized
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return actor, id, nil
}
