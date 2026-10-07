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

// TimerService est ce que le handler attend du chrono.
type TimerService interface {
	Current(ctx context.Context, userID uuid.UUID) (*usecase.Timer, error)
	Start(ctx context.Context, userID uuid.UUID, in usecase.TimerInput) (*usecase.Timer, error)
	Stop(ctx context.Context, userID uuid.UUID) (*usecase.TimeEntry, error)
	Discard(ctx context.Context, userID uuid.UUID) error
}

// Timer sert le chrono de la personne connectee.
type Timer struct {
	svc TimerService
}

func NewTimer(svc TimerService) *Timer { return &Timer{svc: svc} }

// timerResponse enveloppe le chrono : `timer` est nul quand rien ne tourne.
type timerResponse struct {
	Timer *usecase.Timer `json:"timer"`
}

// Get rend le chrono en cours.
func (h *Timer) Get(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	timer, err := h.svc.Current(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(timerResponse{Timer: timer})
}

type startTimerRequest struct {
	ProjectID string  `json:"project_id"`
	TaskID    *string `json:"task_id"`
	Note      string  `json:"note"`
}

// Start lance le chrono, en arretant celui qui tournait.
func (h *Timer) Start(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	var req startTimerRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	projectID, err := uuid.Parse(strings.TrimSpace(req.ProjectID))
	if err != nil {
		return domain.ErrValidation.WithDetails(map[string]any{"project_id": "Identifiant invalide"})
	}

	in := usecase.TimerInput{ProjectID: projectID, Note: strings.TrimSpace(req.Note)}
	if req.TaskID != nil && strings.TrimSpace(*req.TaskID) != "" {
		taskID, err := uuid.Parse(strings.TrimSpace(*req.TaskID))
		if err != nil {
			return domain.ErrValidation.WithDetails(map[string]any{"task_id": "Identifiant invalide"})
		}
		in.TaskID = &taskID
	}

	timer, err := h.svc.Start(c.Context(), userID, in)
	if err != nil {
		return err
	}

	return c.JSON(timerResponse{Timer: timer})
}

// stopTimerResponse rend la saisie ecrite, nulle si rien ne tournait.
type stopTimerResponse struct {
	Entry *usecase.TimeEntry `json:"entry"`
}

// Stop arrete le chrono et ecrit la saisie.
func (h *Timer) Stop(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	entry, err := h.svc.Stop(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(stopTimerResponse{Entry: entry})
}

// Discard abandonne le chrono sans rien ecrire.
func (h *Timer) Discard(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	if err := h.svc.Discard(c.Context(), userID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
