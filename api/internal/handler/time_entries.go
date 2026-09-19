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

// TimeEntryService est le contrat dont l'ecran « Saisie » a besoin.
type TimeEntryService interface {
	Sheet(ctx context.Context, userID uuid.UUID, from, to time.Time) (usecase.TimeSheet, error)
	Create(ctx context.Context, userID uuid.UUID, in usecase.TimeEntryInput) (usecase.TimeEntry, error)
	Update(ctx context.Context, userID, id uuid.UUID, in usecase.TimeEntryInput) (usecase.TimeEntry, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

// TimeEntries porte les endpoints du temps passe.
type TimeEntries struct {
	svc TimeEntryService
}

// NewTimeEntries construit le handler.
func NewTimeEntries(svc TimeEntryService) *TimeEntries {
	return &TimeEntries{svc: svc}
}

// timeEntryBody est ce que le formulaire envoie.
type timeEntryBody struct {
	ProjectID string  `json:"project_id"`
	TaskID    *string `json:"task_id"`
	ServiceID *string `json:"service_id"`
	SpentOn   string  `json:"spent_on"`
	Minutes   int32   `json:"minutes"`
	Note      string  `json:"note"`
}

// input traduit le corps en entree de service, en nommant le champ fautif.
func (b timeEntryBody) input() (usecase.TimeEntryInput, error) {
	in := usecase.TimeEntryInput{Minutes: b.Minutes, Note: b.Note}

	projectID, err := uuid.Parse(strings.TrimSpace(b.ProjectID))
	if err != nil {
		return in, domain.ErrValidation.WithDetails(map[string]any{
			"project_id": "Le projet est requis",
		})
	}
	in.ProjectID = projectID

	// Une tache et un service facultatifs : absents ou vides, ils veulent dire
	// la meme chose — rien a rattacher.
	if b.TaskID != nil && strings.TrimSpace(*b.TaskID) != "" {
		taskID, err := uuid.Parse(strings.TrimSpace(*b.TaskID))
		if err != nil {
			return in, domain.ErrValidation.WithDetails(map[string]any{
				"task_id": "Identifiant de tâche invalide",
			})
		}
		in.TaskID = &taskID
	}

	if b.ServiceID != nil && strings.TrimSpace(*b.ServiceID) != "" {
		serviceID, err := uuid.Parse(strings.TrimSpace(*b.ServiceID))
		if err != nil {
			return in, domain.ErrValidation.WithDetails(map[string]any{
				"service_id": "Identifiant de service invalide",
			})
		}
		in.ServiceID = &serviceID
	}

	spentOn, err := time.Parse("2006-01-02", strings.TrimSpace(b.SpentOn))
	if err != nil {
		return in, domain.ErrValidation.WithDetails(map[string]any{
			"spent_on": "Date attendue au format AAAA-MM-JJ",
		})
	}
	in.SpentOn = spentOn

	return in, nil
}

// Sheet sert l'ecran de saisie : les lignes d'une plage, leur total, et le
// total de chaque jour.
//
// La plage vient de l'adresse et le compte de la session : on saisit et on
// relit son propre temps, jamais celui d'un autre.
func (h *TimeEntries) Sheet(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	// Par defaut, la journee courante : c'est l'ecran qu'on ouvre le soir.
	today := time.Now().Format("2006-01-02")

	from, err := parseDay(c.Query("from", today), "from")
	if err != nil {
		return err
	}

	to, err := parseDay(c.Query("to", c.Query("from", today)), "to")
	if err != nil {
		return err
	}

	sheet, err := h.svc.Sheet(c.Context(), userID, from, to)
	if err != nil {
		return err
	}

	return c.JSON(sheet)
}

// parseDay lit un jour du format AAAA-MM-JJ.
func parseDay(raw, field string) (time.Time, error) {
	day, err := time.Parse("2006-01-02", strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, domain.ErrValidation.WithDetails(map[string]any{
			field: "Date attendue au format AAAA-MM-JJ",
		})
	}

	return day, nil
}

// Create enregistre une saisie.
func (h *TimeEntries) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	var body timeEntryBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	in, err := body.input()
	if err != nil {
		return err
	}

	entry, err := h.svc.Create(c.Context(), userID, in)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(entry)
}

// Update modifie une saisie.
func (h *TimeEntries) Update(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var body timeEntryBody
	if err := c.Bind().Body(&body); err != nil {
		return domain.ErrValidation
	}

	in, err := body.input()
	if err != nil {
		return err
	}

	entry, err := h.svc.Update(c.Context(), userID, id, in)
	if err != nil {
		return err
	}

	return c.JSON(entry)
}

// Delete retire une saisie.
func (h *TimeEntries) Delete(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), userID, id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
