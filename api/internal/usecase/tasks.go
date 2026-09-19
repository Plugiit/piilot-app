package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// Borne du tableau. Un projet qui la depasse a un probleme de decoupage, pas
// d'affichage : l'ecran le dit plutot que de charger indefiniment.
const taskBoardLimit = 300

// Borne des collections du panneau lateral.
const (
	taskCommentsLimit = 100
	taskActivityLimit = 50
)

// TaskSummary est une carte du tableau : ce que la carte montre, rien de plus.
// La note, la description et les sous-taches n'y figurent pas — elles ne sont
// lues qu'a l'ouverture du panneau.
type TaskSummary struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	Tag       string    `json:"tag"`
	Priority  string    `json:"priority"`
	// Ce que la carte du kanban montre sans ouvrir la tache. Compteurs tenus
	// par declencheur en base : aucun COUNT n'est fait au rendu du tableau.
	Description      string   `json:"description"`
	SubtasksTotal    int      `json:"subtasks_total"`
	SubtasksDone     int      `json:"subtasks_done"`
	CommentsCount    int      `json:"comments_count"`
	AttachmentsCount int      `json:"attachments_count"`
	DueOn            *string  `json:"due_on"`
	Hours            *float64 `json:"hours"`
	Position         int      `json:"position"`
	Assignees        []Person `json:"assignees"`
	// Prestations sur lesquelles la tache compte. Vide quand elle n'en releve
	// d'aucune — une reunion interne, un correctif d'intendance — et parfois
	// plusieurs.
	Services []ServiceTag `json:"services"`
}

// TaskBoard est le contenu de l'onglet « Tâches » d'un projet.
type TaskBoard struct {
	Items []TaskSummary `json:"items"`
	Total int64         `json:"total"`
	// Limit dit combien de taches l'ecran a le droit d'afficher : compare a
	// Total, il permet de signaler qu'on n'en montre pas la totalite.
	Limit int `json:"limit"`
}

// TaskListItem est une carte de l'ecran « Taches » du module.
//
// La meme carte que dans un projet, plus le nom de celui-ci : sortie de sa
// fiche, une tache ne dit plus a quoi elle se rattache. TaskSummary est
// embarque plutot que recopie — les deux ecrans dessinent la meme carte, et
// dupliquer quinze champs garantissait qu'ils finiraient par diverger.
type TaskListItem struct {
	TaskSummary
	ProjectName string `json:"project_name"`
}

// TaskList est le contenu de l'ecran « Taches », ses deux vues comprises.
//
// Pas de pagination, une borne : l'ecran a une vue kanban, et un kanban ne se
// feuillette pas. Total dit ce qui existe, Limit ce que la vue a le droit de
// montrer — leur ecart est ce que l'ecran signale.
type TaskList struct {
	Items []TaskListItem `json:"items"`
	Total int64          `json:"total"`
	Limit int            `json:"limit"`
}

// TaskFilters porte ce que la barre d'outils de l'ecran « Taches » sait
// reduire. Tous facultatifs : sans aucun, l'ecran montre l'agence entiere.
type TaskFilters struct {
	Status    *string
	Priority  *string
	ProjectID *uuid.UUID
	Search    *string
}

// Subtask est une ligne de la liste a cocher du panneau.
type Subtask struct {
	ID       uuid.UUID `json:"id"`
	Label    string    `json:"label"`
	Done     bool      `json:"done"`
	Position int       `json:"position"`
}

// Comment est un message de l'onglet « Commentaires ».
type Comment struct {
	ID        uuid.UUID `json:"id"`
	Body      string    `json:"body"`
	Author    *Person   `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// ActivityEntry est une ligne du journal.
//
// Le libelle n'est pas compose ici : le serveur dit ce qui s'est passe (`kind`)
// et avec quoi (`payload`), l'ecran l'ecrit dans sa langue. Composer la phrase
// en Go reviendrait a mettre du francais d'interface dans l'API.
type ActivityEntry struct {
	ID        uuid.UUID      `json:"id"`
	Kind      string         `json:"kind"`
	Payload   map[string]any `json:"payload"`
	Actor     *Person        `json:"actor"`
	CreatedAt time.Time      `json:"created_at"`
}

// TaskDetail est tout ce que le panneau lateral affiche a son ouverture.
//
// Les commentaires n'en font pas partie : ils ont leur onglet, donc leur
// endpoint. Les charger ici ferait payer a chaque ouverture une liste que la
// plupart des consultations ne regardent pas.
type TaskDetail struct {
	ID          uuid.UUID       `json:"id"`
	ProjectID   uuid.UUID       `json:"project_id"`
	ProjectName string          `json:"project_name"`
	ClientName  string          `json:"client_name"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Tag         string          `json:"tag"`
	Priority    string          `json:"priority"`
	Note        string          `json:"note"`
	StartsOn    *string         `json:"starts_on"`
	DueOn       *string         `json:"due_on"`
	Hours       *float64        `json:"hours"`
	CompletedAt *time.Time      `json:"completed_at"`
	CreatedAt   time.Time       `json:"created_at"`
	Assignees   []Person        `json:"assignees"`
	Subtasks    []Subtask       `json:"subtasks"`
	Files       []Attachment    `json:"files"`
	Activity    []ActivityEntry `json:"activity"`
	Services    []ServiceTag    `json:"services"`
}

// CreateTaskInput decrit une tache a creer.
type CreateTaskInput struct {
	ProjectID   uuid.UUID
	Title       string
	Description string
	Status      string
	Tag         string
	Priority    string
	StartsOn    *time.Time
	DueOn       *time.Time
	Hours       *float64
	Note        string
	AssigneeIDs []uuid.UUID
	ActorID     uuid.UUID
	ServiceIDs  []uuid.UUID
}

// UpdateTaskInput ne porte que ce qui change.
type UpdateTaskInput struct {
	Title         *string
	Description   *string
	Tag           *string
	Priority      *string
	Note          *string
	Hours         *float64
	ClearHours    bool
	StartsOn      *time.Time
	ClearStartsOn bool
	DueOn         *time.Time
	ClearDueOn    bool
	// Nul quand le formulaire ne parle pas des services ; une tranche vide les
	// detache tous.
	ServiceIDs *[]uuid.UUID
	ActorID    uuid.UUID
}

// Priorites acceptees. La table du projet dit la meme chose pour les projets ;
// les deux restent separees parce que rien ne garantit qu'elles evolueront
// ensemble.
var taskPriorities = map[string]struct{}{
	"low": {}, "medium": {}, "high": {},
}

var taskStatuses = map[string]struct{}{
	"todo": {}, "progress": {}, "review": {}, "done": {},
}

// TaskService porte les taches, leurs sous-taches, leurs commentaires et leur
// journal.
type TaskService struct {
	// Diffusion des notifications. Nul en test : le service journalise et
	// ecrit en base sans que personne n'ait a ecouter.
	bus     Bus
	pool    *pgxpool.Pool
	q       *db.Queries
	files   storage.Store
	maxFile int64
}

// NewTaskService construit le service.
func NewTaskService(pool *pgxpool.Pool, files storage.Store, maxFile int64, bus Bus) *TaskService {
	return &TaskService{pool: pool, q: db.New(pool), files: files, maxFile: maxFile}
}

// List renvoie les taches de toute l'agence, filtrees par la barre d'outils.
//
// Deux requetes pour la page, plus une pour les affectations : le compte, la
// liste, puis les personnes affectees d'un coup — jamais une requete par
// ligne.
func (s *TaskService) List(ctx context.Context, f TaskFilters) (TaskList, error) {
	total, err := s.q.CountTasks(ctx, db.CountTasksParams{
		Status:    f.Status,
		Priority:  f.Priority,
		ProjectID: f.ProjectID,
		Search:    f.Search,
	})
	if err != nil {
		return TaskList{}, fmt.Errorf("comptage des taches : %w", err)
	}

	rows, err := s.q.ListTasks(ctx, db.ListTasksParams{
		Status:    f.Status,
		Priority:  f.Priority,
		ProjectID: f.ProjectID,
		Search:    f.Search,
		PageSize:  taskBoardLimit,
	})
	if err != nil {
		return TaskList{}, fmt.Errorf("lecture des taches : %w", err)
	}

	items := make([]TaskListItem, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		items = append(items, TaskListItem{
			TaskSummary: TaskSummary{
				ID:        row.ID,
				ProjectID: row.ProjectID,
				Title:     row.Title,
				// Vide et non nulle : le contrat annonce un tableau, et
				// l'ecran qui compte ses elements tomberait sur un nul.
				Services:         []ServiceTag{},
				Status:           row.Status,
				Tag:              row.Tag,
				Priority:         row.Priority,
				Description:      row.Description,
				SubtasksTotal:    int(row.SubtasksTotal),
				SubtasksDone:     int(row.SubtasksDone),
				CommentsCount:    int(row.CommentsCount),
				AttachmentsCount: int(row.AttachmentsCount),
				DueOn:            formatDate(row.DueOn),
				Hours:            row.Hours,
				Position:         int(row.Position),
				Assignees:        []Person{},
			},
			ProjectName: row.ProjectName,
		})
	}

	byTask, err := s.assigneesOf(ctx, ids)
	if err != nil {
		return TaskList{}, err
	}

	byService, err := s.servicesOf(ctx, ids)
	if err != nil {
		return TaskList{}, err
	}

	for i := range items {
		if people, ok := byTask[items[i].ID]; ok {
			items[i].Assignees = people
		}
		if services, ok := byService[items[i].ID]; ok {
			items[i].Services = services
		}
	}

	return TaskList{Items: items, Total: total, Limit: taskBoardLimit}, nil
}

// Board renvoie le tableau d'un projet : deux requetes, quel que soit le
// nombre de colonnes et de cartes.
func (s *TaskService) Board(ctx context.Context, projectID uuid.UUID) (TaskBoard, error) {
	total, err := s.q.CountTasksOfProject(ctx, projectID)
	if err != nil {
		return TaskBoard{}, fmt.Errorf("comptage des taches : %w", err)
	}

	rows, err := s.q.ListTasksOfProject(ctx, db.ListTasksOfProjectParams{
		ProjectID: projectID,
		PageSize:  taskBoardLimit,
	})
	if err != nil {
		return TaskBoard{}, fmt.Errorf("lecture des taches : %w", err)
	}

	items := make([]TaskSummary, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		items = append(items, TaskSummary{
			ID:               row.ID,
			ProjectID:        row.ProjectID,
			Title:            row.Title,
			Services:         []ServiceTag{},
			Status:           row.Status,
			Tag:              row.Tag,
			Priority:         row.Priority,
			Description:      row.Description,
			SubtasksTotal:    int(row.SubtasksTotal),
			SubtasksDone:     int(row.SubtasksDone),
			CommentsCount:    int(row.CommentsCount),
			AttachmentsCount: int(row.AttachmentsCount),
			DueOn:            formatDate(row.DueOn),
			Hours:            row.Hours,
			Position:         int(row.Position),
			Assignees:        []Person{},
		})
	}

	byTask, err := s.assigneesOf(ctx, ids)
	if err != nil {
		return TaskBoard{}, err
	}

	byService, err := s.servicesOf(ctx, ids)
	if err != nil {
		return TaskBoard{}, err
	}

	for i := range items {
		if people, ok := byTask[items[i].ID]; ok {
			items[i].Assignees = people
		}
		if services, ok := byService[items[i].ID]; ok {
			items[i].Services = services
		}
	}

	return TaskBoard{Items: items, Total: total, Limit: taskBoardLimit}, nil
}

// Get renvoie le detail d'une tache : tache, affectations, sous-taches,
// journal.
func (s *TaskService) Get(ctx context.Context, id uuid.UUID) (TaskDetail, error) {
	row, err := s.q.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskDetail{}, domain.ErrNotFound
		}
		return TaskDetail{}, fmt.Errorf("lecture de la tache : %w", err)
	}

	byTask, err := s.assigneesOf(ctx, []uuid.UUID{id})
	if err != nil {
		return TaskDetail{}, err
	}

	byService, err := s.servicesOf(ctx, []uuid.UUID{id})
	if err != nil {
		return TaskDetail{}, err
	}

	subtaskRows, err := s.q.ListSubtasks(ctx, id)
	if err != nil {
		return TaskDetail{}, fmt.Errorf("lecture des sous-taches : %w", err)
	}

	subtasks := make([]Subtask, 0, len(subtaskRows))
	for _, sub := range subtaskRows {
		subtasks = append(subtasks, Subtask{
			ID:       sub.ID,
			Label:    sub.Label,
			Done:     sub.Done,
			Position: int(sub.Position),
		})
	}

	fileRows, err := s.q.ListTaskFiles(ctx, []uuid.UUID{id})
	if err != nil {
		return TaskDetail{}, fmt.Errorf("lecture des pieces jointes : %w", err)
	}

	files := make([]Attachment, 0, len(fileRows))
	for _, file := range fileRows {
		files = append(files, Attachment{
			ID:          file.ID,
			Filename:    file.Filename,
			ContentType: file.ContentType,
			SizeBytes:   file.SizeBytes,
			CreatedAt:   file.CreatedAt,
		})
	}

	activityRows, err := s.q.ListTaskActivity(ctx, db.ListTaskActivityParams{
		TaskID:   id,
		PageSize: taskActivityLimit,
	})
	if err != nil {
		return TaskDetail{}, fmt.Errorf("lecture du journal : %w", err)
	}

	activity := make([]ActivityEntry, 0, len(activityRows))
	for _, entry := range activityRows {
		payload := map[string]any{}
		// Un journal illisible ne doit pas faire echouer l'ouverture du
		// panneau : la ligne passe avec un payload vide.
		_ = json.Unmarshal(entry.Payload, &payload)

		activity = append(activity, ActivityEntry{
			ID:        entry.ID,
			Kind:      entry.Kind,
			Payload:   payload,
			Actor:     personFromNullable(entry.ActorID, entry.Firstname, entry.Lastname, entry.AvatarUrl),
			CreatedAt: entry.CreatedAt,
		})
	}

	assignees := []Person{}
	if people, ok := byTask[id]; ok {
		assignees = people
	}

	return TaskDetail{
		ID:          row.ID,
		ProjectID:   row.ProjectID,
		ProjectName: row.ProjectName,
		ClientName:  row.ClientName,
		Title:       row.Title,
		Description: row.Description,
		Status:      row.Status,
		Tag:         row.Tag,
		Priority:    row.Priority,
		Note:        row.Note,
		StartsOn:    formatDate(row.StartsOn),
		DueOn:       formatDate(row.DueOn),
		Hours:       row.Hours,
		CompletedAt: row.CompletedAt,
		CreatedAt:   row.CreatedAt,
		Assignees:   assignees,
		Services:    servicesOrEmpty(byService[id]),
		Subtasks:    subtasks,
		Files:       files,
		Activity:    activity,
	}, nil
}

// Create cree une tache, ses affectations, et ouvre son journal.
func (s *TaskService) Create(ctx context.Context, in CreateTaskInput) (TaskDetail, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"title": "Le titre de la tâche est requis",
		})
	}

	if in.Status == "" {
		in.Status = "todo"
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if _, ok := taskPriorities[in.Priority]; !ok {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"priority": "Priorité inconnue",
		})
	}
	if _, ok := taskStatuses[in.Status]; !ok {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"status": "Statut inconnu",
		})
	}
	if in.Hours != nil && *in.Hours < 0 {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"hours": "L'estimation ne peut pas être négative",
		})
	}
	if in.StartsOn != nil && in.DueOn != nil && in.DueOn.Before(*in.StartsOn) {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"due_on": "L'échéance précède la date de début",
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TaskDetail{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	task, err := qtx.CreateTask(ctx, db.CreateTaskParams{
		ProjectID:   in.ProjectID,
		Title:       title,
		Description: in.Description,
		Status:      in.Status,
		Tag:         in.Tag,
		Priority:    in.Priority,
		StartsOn:    in.StartsOn,
		DueOn:       in.DueOn,
		Hours:       in.Hours,
		Note:        in.Note,
		CreatedBy:   &in.ActorID,
	})
	if err != nil {
		// Une cle etrangere violee ici, c'est un projet inconnu : le dire
		// plutot que de renvoyer une erreur interne.
		if isForeignKeyViolation(err) {
			return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"project_id": "Projet introuvable",
			})
		}
		return TaskDetail{}, fmt.Errorf("creation de la tache : %w", err)
	}

	for _, userID := range in.AssigneeIDs {
		if err := qtx.AssignTask(ctx, db.AssignTaskParams{TaskID: task.ID, UserID: userID}); err != nil {
			return TaskDetail{}, fmt.Errorf("affectation de la tache : %w", err)
		}
	}

	if err := setTaskServices(ctx, qtx, task.ID, in.ServiceIDs); err != nil {
		return TaskDetail{}, err
	}

	if err := logActivity(ctx, qtx, task.ID, in.ActorID, "created", map[string]any{"title": task.Title}); err != nil {
		return TaskDetail{}, err
	}

	if err := notifyTask(ctx, qtx, s.bus, TaskEvent{
		TaskID:    task.ID,
		ProjectID: task.ProjectID,
		ActorID:   in.ActorID,
		Kind:      NotifyTaskCreated,
		Payload:   map[string]any{"title": task.Title},
	}); err != nil {
		return TaskDetail{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TaskDetail{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, task.ID)
}

// Update modifie les champs libres d'une tache.
//
// Le changement d'echeance est journalise, les autres non : c'est le seul que
// quelqu'un d'autre a besoin de voir passer. Journaliser chaque frappe dans la
// note remplirait le fil sans rien apprendre.
func (s *TaskService) Update(ctx context.Context, id uuid.UUID, in UpdateTaskInput) (TaskDetail, error) {
	if in.Hours != nil && *in.Hours < 0 {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"hours": "L'estimation ne peut pas être négative",
		})
	}
	if in.Priority != nil {
		if _, ok := taskPriorities[*in.Priority]; !ok {
			return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
				"priority": "Priorité inconnue",
			})
		}
	}
	if in.Title != nil && strings.TrimSpace(*in.Title) == "" {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"title": "Le titre de la tâche est requis",
		})
	}

	before, err := s.q.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskDetail{}, domain.ErrNotFound
		}
		return TaskDetail{}, fmt.Errorf("lecture de la tache : %w", err)
	}

	updated, err := s.q.UpdateTask(ctx, db.UpdateTaskParams{
		ID:            id,
		Title:         in.Title,
		Description:   in.Description,
		Tag:           in.Tag,
		Priority:      in.Priority,
		Note:          in.Note,
		Hours:         in.Hours,
		ClearHours:    in.ClearHours,
		StartsOn:      in.StartsOn,
		ClearStartsOn: in.ClearStartsOn,
		DueOn:         in.DueOn,
		ClearDueOn:    in.ClearDueOn,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskDetail{}, domain.ErrNotFound
		}
		return TaskDetail{}, fmt.Errorf("mise a jour de la tache : %w", err)
	}

	// Nul quand le formulaire n'en parle pas : une mise a jour partielle ne
	// doit pas effacer ce qu'elle ignore.
	if in.ServiceIDs != nil {
		if err := setTaskServices(ctx, s.q, id, *in.ServiceIDs); err != nil {
			return TaskDetail{}, err
		}
	}

	if !sameDate(before.DueOn, updated.DueOn) {
		if err := logActivity(ctx, s.q, id, in.ActorID, "due_changed", map[string]any{
			"from": formatDateValue(before.DueOn),
			"to":   formatDateValue(updated.DueOn),
		}); err != nil {
			return TaskDetail{}, err
		}

		if err := notifyTask(ctx, s.q, s.bus, TaskEvent{
			TaskID:    id,
			ProjectID: updated.ProjectID,
			ActorID:   in.ActorID,
			Kind:      NotifyTaskDueChanged,
			Payload: map[string]any{
				"title": updated.Title,
				"from":  formatDateValue(before.DueOn),
				"to":    formatDateValue(updated.DueOn),
			},
		}); err != nil {
			return TaskDetail{}, err
		}
	}

	return s.Get(ctx, id)
}

// Move deplace une tache dans le tableau.
func (s *TaskService) Move(ctx context.Context, id uuid.UUID, status string, position *int, actorID uuid.UUID) (TaskDetail, error) {
	if _, ok := taskStatuses[status]; !ok {
		return TaskDetail{}, domain.ErrValidation.WithDetails(map[string]any{
			"status": "Statut inconnu",
		})
	}

	before, err := s.q.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskDetail{}, domain.ErrNotFound
		}
		return TaskDetail{}, fmt.Errorf("lecture de la tache : %w", err)
	}

	var rank *int32
	if position != nil {
		value := int32(*position)
		rank = &value
	}

	if _, err := s.q.MoveTask(ctx, db.MoveTaskParams{
		ID:       id,
		Status:   status,
		Position: rank,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskDetail{}, domain.ErrNotFound
		}
		return TaskDetail{}, fmt.Errorf("deplacement de la tache : %w", err)
	}

	// Un simple reordonnancement dans la meme colonne n'est pas un evenement :
	// seul le changement de colonne entre au journal.
	if before.Status != status {
		if err := logActivity(ctx, s.q, id, actorID, "status_changed", map[string]any{
			"from": before.Status,
			"to":   status,
		}); err != nil {
			return TaskDetail{}, err
		}

		if err := notifyTask(ctx, s.q, s.bus, TaskEvent{
			TaskID:    id,
			ProjectID: before.ProjectID,
			ActorID:   actorID,
			Kind:      NotifyTaskStatusChanged,
			Payload: map[string]any{
				"title": before.Title,
				"from":  before.Status,
				"to":    status,
			},
		}); err != nil {
			return TaskDetail{}, err
		}
	}

	return s.Get(ctx, id)
}

// Delete efface une tache logiquement.
func (s *TaskService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.q.SoftDeleteTask(ctx, id); err != nil {
		return fmt.Errorf("suppression de la tache : %w", err)
	}
	return nil
}

// SetAssignees remplace les personnes affectees et journalise l'ecart.
func (s *TaskService) SetAssignees(ctx context.Context, id uuid.UUID, userIDs []uuid.UUID, actorID uuid.UUID) (TaskDetail, error) {
	task, err := s.q.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TaskDetail{}, domain.ErrNotFound
		}
		return TaskDetail{}, fmt.Errorf("lecture de la tache : %w", err)
	}

	current, err := s.assigneesOf(ctx, []uuid.UUID{id})
	if err != nil {
		return TaskDetail{}, err
	}

	existing := make(map[uuid.UUID]Person, len(current[id]))
	for _, person := range current[id] {
		existing[person.ID] = person
	}

	wanted := make(map[uuid.UUID]struct{}, len(userIDs))
	for _, userID := range userIDs {
		wanted[userID] = struct{}{}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TaskDetail{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	for personID, person := range existing {
		if _, keep := wanted[personID]; keep {
			continue
		}

		if err := qtx.UnassignTask(ctx, db.UnassignTaskParams{TaskID: id, UserID: personID}); err != nil {
			return TaskDetail{}, fmt.Errorf("retrait d'une affectation : %w", err)
		}
		if err := logActivity(ctx, qtx, id, actorID, "unassigned", map[string]any{
			"user_id": personID,
			"name":    strings.TrimSpace(person.Firstname + " " + person.Lastname),
		}); err != nil {
			return TaskDetail{}, err
		}

		// Destinataire impose : la personne vient d'etre retiree, elle ne
		// figure plus parmi les affectees qu'on irait chercher.
		if err := notifyTask(ctx, qtx, s.bus, TaskEvent{
			TaskID:     id,
			ProjectID:  task.ProjectID,
			ActorID:    actorID,
			Kind:       NotifyTaskUnassigned,
			Payload:    map[string]any{"title": task.Title},
			Recipients: []uuid.UUID{personID},
		}); err != nil {
			return TaskDetail{}, err
		}
	}

	for _, userID := range userIDs {
		if _, already := existing[userID]; already {
			continue
		}

		if err := qtx.AssignTask(ctx, db.AssignTaskParams{TaskID: id, UserID: userID}); err != nil {
			return TaskDetail{}, fmt.Errorf("ajout d'une affectation : %w", err)
		}
		if err := logActivity(ctx, qtx, id, actorID, "assigned", map[string]any{
			"user_id": userID,
		}); err != nil {
			return TaskDetail{}, err
		}

		if err := notifyTask(ctx, qtx, s.bus, TaskEvent{
			TaskID:     id,
			ProjectID:  task.ProjectID,
			ActorID:    actorID,
			Kind:       NotifyTaskAssigned,
			Payload:    map[string]any{"title": task.Title},
			Recipients: []uuid.UUID{userID},
		}); err != nil {
			return TaskDetail{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return TaskDetail{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return s.Get(ctx, id)
}

// AddSubtask ajoute une ligne a cocher.
func (s *TaskService) AddSubtask(ctx context.Context, taskID uuid.UUID, label string) (Subtask, error) {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return Subtask{}, domain.ErrValidation.WithDetails(map[string]any{
			"label": "Le libellé est requis",
		})
	}

	row, err := s.q.CreateSubtask(ctx, db.CreateSubtaskParams{TaskID: taskID, Label: trimmed})
	if err != nil {
		if isForeignKeyViolation(err) {
			return Subtask{}, domain.ErrNotFound
		}
		return Subtask{}, fmt.Errorf("creation de la sous-tache : %w", err)
	}

	return Subtask{ID: row.ID, Label: row.Label, Done: row.Done, Position: int(row.Position)}, nil
}

// UpdateSubtask renomme une sous-tache, la coche ou la deplace.
//
// Cocher entre au journal, renommer non : le premier fait avancer le travail,
// le second corrige une formulation.
func (s *TaskService) UpdateSubtask(ctx context.Context, id uuid.UUID, label *string, done *bool, position *int, actorID uuid.UUID) (Subtask, error) {
	if label != nil && strings.TrimSpace(*label) == "" {
		return Subtask{}, domain.ErrValidation.WithDetails(map[string]any{
			"label": "Le libellé est requis",
		})
	}

	before, err := s.q.GetSubtask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subtask{}, domain.ErrNotFound
		}
		return Subtask{}, fmt.Errorf("lecture de la sous-tache : %w", err)
	}

	var rank *int32
	if position != nil {
		value := int32(*position)
		rank = &value
	}

	row, err := s.q.UpdateSubtask(ctx, db.UpdateSubtaskParams{
		ID:       id,
		Label:    label,
		Done:     done,
		Position: rank,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subtask{}, domain.ErrNotFound
		}
		return Subtask{}, fmt.Errorf("mise a jour de la sous-tache : %w", err)
	}

	if done != nil && before.Done != row.Done {
		kind := "subtask_undone"
		if row.Done {
			kind = "subtask_done"
		}

		if err := logActivity(ctx, s.q, before.TaskID, actorID, kind, map[string]any{
			"label": row.Label,
		}); err != nil {
			return Subtask{}, err
		}
	}

	return Subtask{ID: row.ID, Label: row.Label, Done: row.Done, Position: int(row.Position)}, nil
}

// DeleteSubtask retire une ligne a cocher.
func (s *TaskService) DeleteSubtask(ctx context.Context, id uuid.UUID) error {
	if err := s.q.DeleteSubtask(ctx, id); err != nil {
		return fmt.Errorf("suppression de la sous-tache : %w", err)
	}
	return nil
}

// Comments renvoie le fil de discussion d'une tache.
func (s *TaskService) Comments(ctx context.Context, taskID uuid.UUID) ([]Comment, error) {
	rows, err := s.q.ListTaskComments(ctx, db.ListTaskCommentsParams{
		TaskID:   taskID,
		PageSize: taskCommentsLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("lecture des commentaires : %w", err)
	}

	comments := make([]Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, Comment{
			ID:        row.ID,
			Body:      row.Body,
			Author:    personFromNullable(row.AuthorID, row.Firstname, row.Lastname, row.AvatarUrl),
			CreatedAt: row.CreatedAt,
		})
	}

	return comments, nil
}

// AddComment poste un message et le journalise.
func (s *TaskService) AddComment(ctx context.Context, taskID uuid.UUID, authorID uuid.UUID, body string) (Comment, error) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return Comment{}, domain.ErrValidation.WithDetails(map[string]any{
			"body": "Le commentaire est vide",
		})
	}

	task, err := s.q.GetTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Comment{}, domain.ErrNotFound
		}
		return Comment{}, fmt.Errorf("lecture de la tache : %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Comment{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	row, err := qtx.CreateTaskComment(ctx, db.CreateTaskCommentParams{
		TaskID:   taskID,
		AuthorID: &authorID,
		Body:     trimmed,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return Comment{}, domain.ErrNotFound
		}
		return Comment{}, fmt.Errorf("creation du commentaire : %w", err)
	}

	if err := logActivity(ctx, qtx, taskID, authorID, "commented", map[string]any{}); err != nil {
		return Comment{}, err
	}

	if err := notifyTask(ctx, qtx, s.bus, TaskEvent{
		TaskID:    taskID,
		ProjectID: task.ProjectID,
		ActorID:   authorID,
		Kind:      NotifyTaskCommented,
		Payload:   map[string]any{"title": task.Title, "excerpt": excerpt(trimmed)},
	}); err != nil {
		return Comment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Comment{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return Comment{ID: row.ID, Body: row.Body, CreatedAt: row.CreatedAt}, nil
}

// assigneesOf charge les affectations de plusieurs taches en une requete.
func (s *TaskService) assigneesOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]Person, error) {
	byTask := map[uuid.UUID][]Person{}

	if len(ids) == 0 {
		return byTask, nil
	}

	rows, err := s.q.ListAssigneesOfTasks(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("lecture des affectations : %w", err)
	}

	for _, row := range rows {
		byTask[row.TaskID] = append(byTask[row.TaskID], Person{
			ID:        row.ID,
			Firstname: row.Firstname,
			Lastname:  row.Lastname,
			Initials:  initialsOf(row.Firstname, row.Lastname),
			AvatarURL: row.AvatarUrl,
		})
	}

	return byTask, nil
}

// servicesOf charge les services de plusieurs taches en une requete.
//
// Jumeau de assigneesOf, et pour la meme raison : une collection jointe a la
// liste en multiplierait les lignes.
func (s *TaskService) servicesOf(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID][]ServiceTag, error) {
	byTask := map[uuid.UUID][]ServiceTag{}

	if len(ids) == 0 {
		return byTask, nil
	}

	rows, err := s.q.ListServicesOfTasks(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("lecture des services : %w", err)
	}

	for _, row := range rows {
		byTask[row.TaskID] = append(byTask[row.TaskID], ServiceTag{
			ID: row.ID, Name: row.Name, Color: row.Color,
		})
	}

	return byTask, nil
}

// servicesOrEmpty rend une tranche vide plutot que nulle.
//
// Une tache sans service n'a pas d'entree dans la carte des services : la
// lecture rend alors le zero du type, c'est-a-dire nil, qui se serialise en
// `null`. Le contrat annonce un tableau, et l'ecran qui en compte les elements
// tombe sur un nul.
func servicesOrEmpty(services []ServiceTag) []ServiceTag {
	if services == nil {
		return []ServiceTag{}
	}

	return services
}

// setTaskServices remplace les services d'une tache par la liste fournie.
func setTaskServices(ctx context.Context, q *db.Queries, taskID uuid.UUID, ids []uuid.UUID) error {
	if err := q.ClearTaskServices(ctx, taskID); err != nil {
		return fmt.Errorf("remise a zero des services : %w", err)
	}

	for _, serviceID := range ids {
		if err := q.AddTaskService(ctx, db.AddTaskServiceParams{
			TaskID:    taskID,
			ServiceID: serviceID,
		}); err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrValidation.WithDetails(map[string]any{
					"service_ids": "Un des services n'existe pas",
				})
			}

			return fmt.Errorf("affectation des services : %w", err)
		}
	}

	return nil
}

// logActivity ecrit une ligne de journal.
//
// Prend un *db.Queries plutot que de passer par le service : l'appelant decide
// s'il ecrit dans une transaction ou hors d'elle, et une entree de journal doit
// vivre dans la meme transaction que le fait qu'elle raconte.
func logActivity(ctx context.Context, q *db.Queries, taskID, actorID uuid.UUID, kind string, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encodage du journal : %w", err)
	}

	if err := q.LogTaskActivity(ctx, db.LogTaskActivityParams{
		TaskID:  taskID,
		ActorID: &actorID,
		Kind:    kind,
		Payload: encoded,
	}); err != nil {
		return fmt.Errorf("ecriture du journal : %w", err)
	}

	return nil
}

// personFromNullable reconstruit un auteur dont le compte a pu etre supprime.
// Un commentaire survit a son auteur : la ligne reste, sans nom.
func personFromNullable(id *uuid.UUID, firstname, lastname, avatar *string) *Person {
	if id == nil {
		return nil
	}

	person := Person{ID: *id, AvatarURL: avatar}
	if firstname != nil {
		person.Firstname = *firstname
	}
	if lastname != nil {
		person.Lastname = *lastname
	}
	person.Initials = initialsOf(person.Firstname, person.Lastname)

	return &person
}

// sameDate compare deux echeances, nil compris.
func sameDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// formatDateValue rend une date pour le journal : une chaine, ou nil.
func formatDateValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(dateLayout)
}

// AddFile attache un fichier a une tache.
//
// Meme deroule que pour un projet : le fichier part sur le disque avant la
// ligne, parce qu'on ne connait sa taille qu'une fois ecrit, et il est efface
// si l'insertion echoue. Le compteur de la carte est tenu par declencheur.
func (s *TaskService) AddFile(
	ctx context.Context,
	taskID, uploader uuid.UUID,
	filename, contentType string,
	content io.Reader,
) (Attachment, error) {
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" || filename == "." || filename == "/" {
		return Attachment{}, domain.ErrValidation.WithDetails(map[string]any{
			"file": "Nom de fichier invalide",
		})
	}

	if _, err := s.q.GetTask(ctx, taskID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Attachment{}, domain.ErrNotFound
		}
		return Attachment{}, fmt.Errorf("lecture de la tache : %w", err)
	}

	key, size, err := s.files.Save(content, s.maxFile)
	if err != nil {
		if errors.Is(err, storage.ErrTooLarge) {
			return Attachment{}, domain.ErrValidation.WithDetails(map[string]any{
				"file": fmt.Sprintf("Le fichier dépasse %d Mo", s.maxFile/(1<<20)),
			})
		}
		return Attachment{}, fmt.Errorf("ecriture de la piece jointe : %w", err)
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	row, err := s.q.CreateTaskFile(ctx, db.CreateTaskFileParams{
		TaskID:      &taskID,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   size,
		StorageKey:  key,
		UploadedBy:  &uploader,
	})
	if err != nil {
		_ = s.files.Remove(key)
		return Attachment{}, fmt.Errorf("enregistrement de la piece jointe : %w", err)
	}

	return Attachment{
		ID:          row.ID,
		Filename:    row.Filename,
		ContentType: row.ContentType,
		SizeBytes:   row.SizeBytes,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// excerpt raccourcit un commentaire pour la ligne de notification.
//
// Le panneau annonce ce qui s'est passe ; lire le commentaire entier se fait
// dans la tache. Coupe sur un mot plutot qu'au milieu d'un caractere, les
// accents comptant pour plusieurs octets.
func excerpt(body string) string {
	const limit = 120

	runes := []rune(body)
	if len(runes) <= limit {
		return body
	}

	cut := string(runes[:limit])
	if space := strings.LastIndex(cut, " "); space > limit/2 {
		cut = cut[:space]
	}

	return cut + "…"
}
