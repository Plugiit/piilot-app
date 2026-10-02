package handler

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/middleware"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// MilestoneService est le contrat des jalons et du planning.
type MilestoneService interface {
	List(ctx context.Context, projectID uuid.UUID) (usecase.MilestoneList, error)
	Create(ctx context.Context, projectID uuid.UUID, in usecase.MilestoneInput, createdBy uuid.UUID) (usecase.Milestone, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.MilestoneInput) (usecase.Milestone, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AttachDeliverable(ctx context.Context, deliverableID uuid.UUID, milestoneID *uuid.UUID) error
	Planning(ctx context.Context, userID uuid.UUID, from, to time.Time, mine bool) (usecase.Planning, error)
}

// Milestones porte les jalons et le planning.
type Milestones struct {
	svc MilestoneService
}

// NewMilestones construit le handler.
func NewMilestones(svc MilestoneService) *Milestones {
	return &Milestones{svc: svc}
}

// milestoneBody est ce que l'onglet « Jalons » envoie. Pour une modification,
// une cle absente laisse le champ en place.
type milestoneBody struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	DueOn       *string `json:"due_on"`
	Done        *bool   `json:"done"`
}

func (b milestoneBody) input(raw []byte) (usecase.MilestoneInput, error) {
	in := usecase.MilestoneInput{Title: b.Title, Description: b.Description}

	if hasJSONKey(raw, "due_on") {
		due, err := parseDatePointer(b.DueOn, "due_on")
		if err != nil {
			return in, err
		}
		in.SetDue = true
		in.DueOn = due
	}
	if b.Done != nil {
		in.SetDone = true
		in.Done = *b.Done
	}

	return in, nil
}

// List sert l'onglet « Jalons » d'un projet.
func (h *Milestones) List(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	list, err := h.svc.List(c.Context(), projectID)
	if err != nil {
		return err
	}

	return c.JSON(list)
}

// Create ajoute un jalon au projet.
func (h *Milestones) Create(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body milestoneBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in, err := body.input(c.Body())
	if err != nil {
		return err
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	milestone, err := h.svc.Create(c.Context(), projectID, in, actor)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(milestone)
}

// Update modifie un jalon, ou le marque atteint.
func (h *Milestones) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body milestoneBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	in, err := body.input(c.Body())
	if err != nil {
		return err
	}

	milestone, err := h.svc.Update(c.Context(), id, in)
	if err != nil {
		return err
	}

	return c.JSON(milestone)
}

// Delete supprime un jalon.
func (h *Milestones) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// AttachDeliverable rattache un livrable a un jalon, ou l'en detache.
func (h *Milestones) AttachDeliverable(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body struct {
		MilestoneID *string `json:"milestone_id"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	var milestoneID *uuid.UUID
	if body.MilestoneID != nil && strings.TrimSpace(*body.MilestoneID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*body.MilestoneID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"milestone_id": "Identifiant invalide"})
		}
		milestoneID = &parsed
	}

	if err := h.svc.AttachDeliverable(c.Context(), id, milestoneID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Planning sert le calendrier d'une periode.
func (h *Milestones) Planning(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	fromRaw, toRaw := c.Query("from"), c.Query("to")
	from, err := parseDatePointer(&fromRaw, "from")
	if err != nil {
		return err
	}
	to, err := parseDatePointer(&toRaw, "to")
	if err != nil {
		return err
	}
	if from == nil || to == nil {
		return domain.ErrValidation.WithDetails(map[string]any{"from": "La période est requise"})
	}

	scope := c.Query("scope", "all")
	if scope != "all" && scope != "mine" {
		return domain.ErrValidation.WithDetails(map[string]any{"scope": "Valeur attendue : all ou mine"})
	}

	planning, err := h.svc.Planning(c.Context(), userID, *from, *to, scope == "mine")
	if err != nil {
		return err
	}

	return c.JSON(planning)
}
