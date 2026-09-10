package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/plugiit/plugiit-api-go/internal/domain"
	"github.com/plugiit/plugiit-api-go/internal/middleware"
	"github.com/plugiit/plugiit-api-go/internal/usecase"
)

// TaskService est le contrat dont les endpoints de taches ont besoin.
type TaskService interface {
	Board(ctx context.Context, projectID uuid.UUID) (usecase.TaskBoard, error)
	Get(ctx context.Context, id uuid.UUID) (usecase.TaskDetail, error)
	Create(ctx context.Context, in usecase.CreateTaskInput) (usecase.TaskDetail, error)
	Update(ctx context.Context, id uuid.UUID, in usecase.UpdateTaskInput) (usecase.TaskDetail, error)
	Move(ctx context.Context, id uuid.UUID, status string, position *int, actorID uuid.UUID) (usecase.TaskDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetAssignees(ctx context.Context, id uuid.UUID, userIDs []uuid.UUID, actorID uuid.UUID) (usecase.TaskDetail, error)
	AddSubtask(ctx context.Context, taskID uuid.UUID, label string) (usecase.Subtask, error)
	UpdateSubtask(ctx context.Context, id uuid.UUID, label *string, done *bool, position *int, actorID uuid.UUID) (usecase.Subtask, error)
	DeleteSubtask(ctx context.Context, id uuid.UUID) error
	Comments(ctx context.Context, taskID uuid.UUID) ([]usecase.Comment, error)
	AddComment(ctx context.Context, taskID, authorID uuid.UUID, body string) (usecase.Comment, error)
}

// Tasks porte les endpoints des taches.
type Tasks struct {
	svc TaskService
}

// NewTasks construit le handler.
func NewTasks(svc TaskService) *Tasks {
	return &Tasks{svc: svc}
}

type createTaskRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tag         string   `json:"tag"`
	StartsOn    *string  `json:"starts_on"`
	DueOn       *string  `json:"due_on"`
	Hours       *float64 `json:"hours"`
	Note        string   `json:"note"`
	AssigneeIDs []string `json:"assignee_ids"`
}

type updateTaskRequest struct {
	Title       *string  `json:"title"`
	Description *string  `json:"description"`
	Tag         *string  `json:"tag"`
	Note        *string  `json:"note"`
	Hours       *float64 `json:"hours"`
	StartsOn    *string  `json:"starts_on"`
	DueOn       *string  `json:"due_on"`
}

type moveTaskRequest struct {
	Status   string `json:"status"`
	Position *int   `json:"position"`
}

type assigneesRequest struct {
	UserIDs []string `json:"user_ids"`
}

type createSubtaskRequest struct {
	Label string `json:"label"`
}

type updateSubtaskRequest struct {
	Label    *string `json:"label"`
	Done     *bool   `json:"done"`
	Position *int    `json:"position"`
}

type createCommentRequest struct {
	Body string `json:"body"`
}

// Board sert l'onglet « Tâches » d'un projet.
func (h *Tasks) Board(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	board, err := h.svc.Board(c.Context(), projectID)
	if err != nil {
		return err
	}

	return c.JSON(board)
}

// Create ajoute une tache au projet.
func (h *Tasks) Create(c fiber.Ctx) error {
	projectID, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req createTaskRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	starts, err := parseDatePointer(req.StartsOn, "starts_on")
	if err != nil {
		return err
	}
	due, err := parseDatePointer(req.DueOn, "due_on")
	if err != nil {
		return err
	}
	assignees, err := parseUUIDs(req.AssigneeIDs, "assignee_ids")
	if err != nil {
		return err
	}

	task, err := h.svc.Create(c.Context(), usecase.CreateTaskInput{
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Tag:         req.Tag,
		StartsOn:    starts,
		DueOn:       due,
		Hours:       req.Hours,
		Note:        req.Note,
		AssigneeIDs: assignees,
		ActorID:     actor,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

// Get sert le panneau de detail d'une tache.
func (h *Tasks) Get(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	task, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(task)
}

// Update modifie les champs libres d'une tache.
func (h *Tasks) Update(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req updateTaskRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	in := usecase.UpdateTaskInput{
		Title:       req.Title,
		Description: req.Description,
		Tag:         req.Tag,
		Note:        req.Note,
		Hours:       req.Hours,
		ActorID:     actor,
	}

	// Meme distinction que sur les projets : une cle a null efface, une cle
	// absente laisse en place.
	body := c.Body()
	if hasJSONKey(body, "hours") && req.Hours == nil {
		in.ClearHours = true
	}
	if hasJSONKey(body, "starts_on") {
		if req.StartsOn == nil {
			in.ClearStartsOn = true
		} else if starts, err := parseDatePointer(req.StartsOn, "starts_on"); err != nil {
			return err
		} else {
			in.StartsOn = starts
		}
	}
	if hasJSONKey(body, "due_on") {
		if req.DueOn == nil {
			in.ClearDueOn = true
		} else if due, err := parseDatePointer(req.DueOn, "due_on"); err != nil {
			return err
		} else {
			in.DueOn = due
		}
	}

	task, err := h.svc.Update(c.Context(), id, in)
	if err != nil {
		return err
	}

	return c.JSON(task)
}

// Move deplace une tache dans le tableau.
func (h *Tasks) Move(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req moveTaskRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	task, err := h.svc.Move(c.Context(), id, req.Status, req.Position, actor)
	if err != nil {
		return err
	}

	return c.JSON(task)
}

// Delete efface une tache.
func (h *Tasks) Delete(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// SetAssignees remplace les personnes affectees a une tache.
func (h *Tasks) SetAssignees(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req assigneesRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	ids, err := parseUUIDs(req.UserIDs, "user_ids")
	if err != nil {
		return err
	}

	task, err := h.svc.SetAssignees(c.Context(), id, ids, actor)
	if err != nil {
		return err
	}

	return c.JSON(task)
}

// AddSubtask ajoute une ligne a cocher.
func (h *Tasks) AddSubtask(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req createSubtaskRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	subtask, err := h.svc.AddSubtask(c.Context(), id, req.Label)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(subtask)
}

// UpdateSubtask renomme, coche ou deplace une sous-tache.
func (h *Tasks) UpdateSubtask(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req updateSubtaskRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	subtask, err := h.svc.UpdateSubtask(c.Context(), id, req.Label, req.Done, req.Position, actor)
	if err != nil {
		return err
	}

	return c.JSON(subtask)
}

// DeleteSubtask retire une ligne a cocher.
func (h *Tasks) DeleteSubtask(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	if err := h.svc.DeleteSubtask(c.Context(), id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Comments sert l'onglet « Commentaires ».
func (h *Tasks) Comments(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	comments, err := h.svc.Comments(c.Context(), id)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"items": comments})
}

// AddComment poste un message sur une tache.
func (h *Tasks) AddComment(c fiber.Ctx) error {
	id, err := pathUUID(c, "id")
	if err != nil {
		return err
	}

	var req createCommentRequest
	if err := c.Bind().Body(&req); err != nil {
		return domain.ErrValidation.WithCause(err)
	}

	actor, ok := middleware.UserIDFrom(c)
	if !ok {
		return domain.ErrUnauthorized
	}

	comment, err := h.svc.AddComment(c.Context(), id, actor, req.Body)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(comment)
}
