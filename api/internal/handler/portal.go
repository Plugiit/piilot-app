package handler

import (
	"context"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// PortalService est le contrat du portail client. Chaque methode recoit
// l'appelant : c'est lui, et lui seul, qui decide de ce qui est visible.
type PortalService interface {
	Projects(ctx context.Context, userID uuid.UUID) (usecase.PortalProjectList, error)
	Project(ctx context.Context, userID, projectID uuid.UUID) (usecase.PortalProjectDetail, error)
	Deliverable(ctx context.Context, userID, deliverableID uuid.UUID) (usecase.PortalDeliverableDetail, error)
	Decide(ctx context.Context, userID, deliverableID uuid.UUID, decision, feedback string) (usecase.PortalDeliverableDetail, error)
	Review(ctx context.Context, deliverableID uuid.UUID, token string) (usecase.PortalReview, error)
	DecideByLink(ctx context.Context, deliverableID uuid.UUID, token, decision, feedback string) (usecase.PortalReview, error)
	OpenFile(ctx context.Context, userID, fileID uuid.UUID) (usecase.Attachment, io.ReadCloser, error)

	Tickets(ctx context.Context, userID uuid.UUID, open *bool, page int) (usecase.PortalTicketPage, error)
	Ticket(ctx context.Context, userID, ticketID uuid.UUID) (usecase.PortalTicketDetail, error)
	CreateTicket(ctx context.Context, userID uuid.UUID, in usecase.PortalTicketInput) (usecase.PortalTicketDetail, error)
	Reply(ctx context.Context, userID, ticketID uuid.UUID, body string) (usecase.PortalTicketDetail, error)
	AttachToTicket(ctx context.Context, userID, ticketID uuid.UUID, filename, contentType string, content io.Reader) (usecase.PortalFile, error)
}

// Portal porte les endpoints du portail client.
type Portal struct {
	svc PortalService
}

// NewPortal construit le handler.
func NewPortal(svc PortalService) *Portal {
	return &Portal{svc: svc}
}

// caller lit l'appelant ; la garde du groupe a deja verifie son role.
func caller(c fiber.Ctx) (uuid.UUID, error) {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return uuid.Nil, domain.ErrUnauthorized
	}

	return userID, nil
}

// Projects sert « Mes projets ».
func (h *Portal) Projects(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}

	list, err := h.svc.Projects(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(list)
}

// Project sert la page d'un projet.
func (h *Portal) Project(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	project, err := h.svc.Project(c.Context(), userID, id)
	if err != nil {
		return err
	}

	return c.JSON(project)
}

// Deliverable sert la page d'un livrable.
func (h *Portal) Deliverable(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	deliverable, err := h.svc.Deliverable(c.Context(), userID, id)
	if err != nil {
		return err
	}

	return c.JSON(deliverable)
}

// Decide enregistre la reponse du client sur la version courante.
func (h *Portal) Decide(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body struct {
		Decision string `json:"decision"`
		Feedback string `json:"feedback"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	deliverable, err := h.svc.Decide(c.Context(), userID, id, body.Decision, body.Feedback)
	if err != nil {
		return err
	}

	return c.JSON(deliverable)
}

// Review sert la page de reponse ouverte depuis l'e-mail. Sans session : le
// jeton signe du lien tient lieu de preuve. Rien n'est decide ici.
func (h *Portal) Review(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	review, err := h.svc.Review(c.Context(), id, c.Query("token"))
	if err != nil {
		return err
	}

	return c.JSON(review)
}

// ReviewDecide enregistre la reponse donnee depuis la page du lien.
func (h *Portal) ReviewDecide(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body struct {
		Token    string `json:"token"`
		Decision string `json:"decision"`
		Feedback string `json:"feedback"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	review, err := h.svc.DecideByLink(c.Context(), id, body.Token, body.Decision, body.Feedback)
	if err != nil {
		return err
	}

	return c.JSON(review)
}

// DownloadFile sert un fichier partage, ou celui d'une version de livrable.
func (h *Portal) DownloadFile(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	file, content, err := h.svc.OpenFile(c.Context(), userID, id)
	if err != nil {
		return err
	}

	// Fiber referme le lecteur une fois le corps ecrit : voir
	// Projects.DownloadFile.
	c.Set(fiber.HeaderContentType, file.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderContentDisposition, contentDisposition(file.Filename))

	return c.SendStream(content, int(file.SizeBytes))
}

// Tickets sert « Support » : les demandes du client, ouvertes ou closes.
func (h *Portal) Tickets(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}

	var open *bool
	switch c.Query("state") {
	case "open":
		v := true
		open = &v
	case "closed":
		v := false
		open = &v
	case "":
	default:
		return domain.ErrValidation.WithDetails(map[string]any{"state": "Valeur attendue : open ou closed"})
	}

	page, err := h.svc.Tickets(c.Context(), userID, open, queryInt(c, "page", 1))
	if err != nil {
		return err
	}

	return c.JSON(page)
}

// Ticket sert une demande et son fil public.
func (h *Portal) Ticket(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	ticket, err := h.svc.Ticket(c.Context(), userID, id)
	if err != nil {
		return err
	}

	return c.JSON(ticket)
}

// CreateTicket depose une demande.
func (h *Portal) CreateTicket(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}

	var body struct {
		ProjectID   string `json:"project_id"`
		Tracker     string `json:"tracker"`
		Priority    string `json:"priority"`
		Subject     string `json:"subject"`
		Description string `json:"description"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	projectID, err := uuid.Parse(body.ProjectID)
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"project_id": "Choisissez un de vos projets"})
	}

	ticket, err := h.svc.CreateTicket(c.Context(), userID, usecase.PortalTicketInput{
		ProjectID: projectID, Tracker: body.Tracker, Priority: body.Priority,
		Subject: body.Subject, Description: body.Description,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(ticket)
}

// Reply ajoute la reponse du client au fil.
func (h *Portal) Reply(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body struct {
		Body string `json:"body"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	ticket, err := h.svc.Reply(c.Context(), userID, id, body.Body)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(ticket)
}

// AttachToTicket recoit une piece jointe d'une demande.
func (h *Portal) AttachToTicket(c fiber.Ctx) error {
	userID, err := caller(c)
	if err != nil {
		return err
	}
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	header, err := c.FormFile("file")
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"file": "Aucun fichier reçu sous le champ « file »"})
	}
	content, err := header.Open()
	if err != nil {
		return fmt.Errorf("lecture du fichier envoye : %w", err)
	}
	defer func() { _ = content.Close() }()

	file, err := h.svc.AttachToTicket(c.Context(), userID, id, header.Filename, header.Header.Get("Content-Type"), content)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(file)
}
