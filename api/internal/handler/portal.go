package handler

import (
	"context"
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
	OpenFile(ctx context.Context, userID, fileID uuid.UUID) (usecase.Attachment, io.ReadCloser, error)
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
