package handler

import (
	"context"
	"mime/multipart"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// InboundService est ce que le handler attend de l'e-mail entrant.
type InboundService interface {
	Settings(ctx context.Context) (usecase.InboundSettingsView, error)
	UpdateSettings(ctx context.Context, in usecase.InboundSettingsInput) (usecase.InboundSettingsView, error)
	RotateWebhookSecret(ctx context.Context) (usecase.InboundSettingsView, error)
	RequestPoll(ctx context.Context) (usecase.InboundSettingsView, error)
	Webhook(ctx context.Context, provider, token string, body []byte, form *multipart.Form) (int, error)
	Held(ctx context.Context, page int) (usecase.InboundPage, error)
	Detail(ctx context.Context, id uuid.UUID) (usecase.InboundDetail, error)
	OpenTicket(ctx context.Context, id, projectID uuid.UUID) (uuid.UUID, error)
	AttachToTicket(ctx context.Context, id uuid.UUID, numero int64) (uuid.UUID, error)
	Dismiss(ctx context.Context, id uuid.UUID) error
}

// Inbound porte l'e-mail entrant : reglages, webhooks, tri.
type Inbound struct {
	svc InboundService
}

// NewInbound construit le handler.
func NewInbound(svc InboundService) *Inbound { return &Inbound{svc: svc} }

// Settings rend les reglages et l'etat de la releve.
func (h *Inbound) Settings(c fiber.Ctx) error {
	out, err := h.svc.Settings(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

type inboundSettingsRequest struct {
	Address      string  `json:"address"`
	IMAPEnabled  bool    `json:"imap_enabled"`
	IMAPHost     string  `json:"imap_host"`
	IMAPPort     int     `json:"imap_port"`
	IMAPSecurity string  `json:"imap_security"`
	IMAPUsername string  `json:"imap_username"`
	IMAPPassword *string `json:"imap_password"`
	IMAPFolder   string  `json:"imap_folder"`
}

// UpdateSettings enregistre les reglages faits a l'ecran.
func (h *Inbound) UpdateSettings(c fiber.Ctx) error {
	var body inboundSettingsRequest
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}
	out, err := h.svc.UpdateSettings(c.Context(), usecase.InboundSettingsInput{
		Address: body.Address, IMAPEnabled: body.IMAPEnabled, IMAPHost: body.IMAPHost, IMAPPort: body.IMAPPort,
		IMAPSecurity: body.IMAPSecurity, IMAPUsername: body.IMAPUsername, IMAPPassword: body.IMAPPassword,
		IMAPFolder: body.IMAPFolder,
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Rotate tire un nouveau secret de webhook.
func (h *Inbound) Rotate(c fiber.Ctx) error {
	out, err := h.svc.RotateWebhookSecret(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Poll demande une releve immediate. 202 : la tache de fond la fait dans les
// dix secondes, l'ecran relit l'etat.
func (h *Inbound) Poll(c fiber.Ctx) error {
	out, err := h.svc.RequestPoll(c.Context())
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(out)
}

// Webhook recoit les e-mails qu'un fournisseur pousse. Sans session : le
// jeton de l'adresse fait office de preuve. L'e-mail est depose et la
// reponse part aussitot ; le rangement se fait en tache de fond.
func (h *Inbound) Webhook(c fiber.Ctx) error {
	provider := c.Params("provider")
	var form *multipart.Form
	if strings.HasPrefix(c.Get(fiber.HeaderContentType), "multipart/form-data") {
		f, err := c.MultipartForm()
		if err != nil {
			return domain.ErrValidation.WithCause(err)
		}
		form = f
	}
	stored, err := h.svc.Webhook(c.Context(), provider, c.Query("token"), c.Body(), form)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"stored": stored})
}

// Held rend une page des e-mails a trier.
func (h *Inbound) Held(c fiber.Ctx) error {
	out, err := h.svc.Held(c.Context(), queryInt(c, "page", 1))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Detail rend un e-mail a trier en entier.
func (h *Inbound) Detail(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	out, err := h.svc.Detail(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// OpenTicket fait d'un e-mail un ticket sur le projet choisi.
func (h *Inbound) OpenTicket(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		ProjectID uuid.UUID `json:"project_id"`
	}
	if err := c.Bind().Body(&body); err != nil || body.ProjectID == uuid.Nil {
		return domain.ErrValidation.WithDetails(map[string]any{"project_id": "Choisissez un projet"})
	}
	ticketID, err := h.svc.OpenTicket(c.Context(), id, body.ProjectID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ticket_id": ticketID})
}

// Attach ajoute un e-mail au fil d'un ticket.
func (h *Inbound) Attach(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	var body struct {
		Numero int64 `json:"numero"`
	}
	if err := c.Bind().Body(&body); err != nil || body.Numero <= 0 {
		return domain.ErrValidation.WithDetails(map[string]any{"numero": "Numéro de ticket attendu"})
	}
	ticketID, err := h.svc.AttachToTicket(c.Context(), id, body.Numero)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ticket_id": ticketID})
}

// Dismiss ecarte un e-mail a trier.
func (h *Inbound) Dismiss(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.Dismiss(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
