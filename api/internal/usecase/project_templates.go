package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Bornes du contenu d'un modele : de quoi decrire un projet type, pas un
// backlog entier.
const (
	templateMaxMilestones = 50
	templateMaxTasks      = 200
)

// TemplateSummary est une ligne de l'ecran « Modeles de projet ».
type TemplateSummary struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Milestones  int       `json:"milestones"`
	Tasks       int       `json:"tasks"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TemplatePage est une page de modeles.
type TemplatePage struct {
	Items    []TemplateSummary `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// TemplateMilestone est un jalon type : un titre et un ecart en jours.
type TemplateMilestone struct {
	Title      string `json:"title"`
	OffsetDays int    `json:"offset_days"`
}

// TemplateTask est une tache type.
type TemplateTask struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	// Echeance en jours apres le debut du projet ; nulle, sans echeance.
	OffsetDays *int `json:"offset_days"`
}

// ProjectTemplate est un modele et tout son contenu.
type ProjectTemplate struct {
	ID          uuid.UUID           `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Services    []ServiceTag        `json:"services"`
	Milestones  []TemplateMilestone `json:"milestones"`
	Tasks       []TemplateTask      `json:"tasks"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// TemplateInput est un modele tel que l'editeur l'enregistre : d'un bloc.
type TemplateInput struct {
	Name        string
	Description string
	ServiceIDs  []uuid.UUID
	Milestones  []TemplateMilestone
	Tasks       []TemplateTask
}

// ProjectTemplateService sert les modeles de projet.
type ProjectTemplateService struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewProjectTemplateService(pool *pgxpool.Pool) *ProjectTemplateService {
	return &ProjectTemplateService{pool: pool, q: db.New(pool)}
}

// List rend une page de modeles.
func (s *ProjectTemplateService) List(ctx context.Context, page, pageSize int) (TemplatePage, error) {
	page, pageSize = clampPage(page, pageSize)

	rows, err := s.q.ListProjectTemplates(ctx, db.ListProjectTemplatesParams{
		PageSize:   int32(pageSize),
		PageOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return TemplatePage{}, fmt.Errorf("lecture des modeles : %w", err)
	}

	total, err := s.q.CountProjectTemplates(ctx)
	if err != nil {
		return TemplatePage{}, fmt.Errorf("comptage des modeles : %w", err)
	}

	items := make([]TemplateSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, TemplateSummary{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Milestones:  int(row.Milestones),
			Tasks:       int(row.Tasks),
			UpdatedAt:   row.UpdatedAt,
		})
	}

	return TemplatePage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// Get rend un modele et son contenu.
func (s *ProjectTemplateService) Get(ctx context.Context, id uuid.UUID) (ProjectTemplate, error) {
	return s.get(ctx, s.q, id)
}

func (s *ProjectTemplateService) get(ctx context.Context, q *db.Queries, id uuid.UUID) (ProjectTemplate, error) {
	row, err := q.GetProjectTemplate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProjectTemplate{}, domain.ErrNotFound
		}

		return ProjectTemplate{}, fmt.Errorf("lecture du modele : %w", err)
	}

	services, err := q.ListTemplateServices(ctx, id)
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("lecture des services du modele : %w", err)
	}
	milestones, err := q.ListTemplateMilestones(ctx, id)
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("lecture des jalons du modele : %w", err)
	}
	tasks, err := q.ListTemplateTasks(ctx, id)
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("lecture des taches du modele : %w", err)
	}

	out := ProjectTemplate{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Services:    make([]ServiceTag, 0, len(services)),
		Milestones:  make([]TemplateMilestone, 0, len(milestones)),
		Tasks:       make([]TemplateTask, 0, len(tasks)),
		UpdatedAt:   row.UpdatedAt,
	}
	for _, sv := range services {
		out.Services = append(out.Services, ServiceTag{ID: sv.ID, Name: sv.Name, Color: sv.Color})
	}
	for _, m := range milestones {
		out.Milestones = append(out.Milestones, TemplateMilestone{Title: m.Title, OffsetDays: int(m.OffsetDays)})
	}
	for _, t := range tasks {
		var offset *int
		if t.OffsetDays != nil {
			v := int(*t.OffsetDays)
			offset = &v
		}
		out.Tasks = append(out.Tasks, TemplateTask{
			Title: t.Title, Description: t.Description, Priority: t.Priority, OffsetDays: offset,
		})
	}

	return out, nil
}

// clean valide et normalise un modele avant ecriture, en nommant la ligne
// fautive.
func (in TemplateInput) clean() (TemplateInput, error) {
	out := in
	out.Name = strings.TrimSpace(in.Name)
	out.Description = strings.TrimSpace(in.Description)

	if out.Name == "" {
		return out, domain.ErrValidation.WithDetails(map[string]any{"name": "Le nom est requis"})
	}
	if len(in.Milestones) > templateMaxMilestones {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"milestones": fmt.Sprintf("Un modèle compte au plus %d jalons", templateMaxMilestones),
		})
	}
	if len(in.Tasks) > templateMaxTasks {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"tasks": fmt.Sprintf("Un modèle compte au plus %d tâches", templateMaxTasks),
		})
	}

	out.Milestones = make([]TemplateMilestone, 0, len(in.Milestones))
	for i, m := range in.Milestones {
		m.Title = strings.TrimSpace(m.Title)
		if m.Title == "" {
			return out, domain.ErrValidation.WithDetails(map[string]any{
				"milestones": fmt.Sprintf("Le jalon n° %d n'a pas de titre", i+1),
			})
		}
		if m.OffsetDays < 0 || m.OffsetDays > 3650 {
			return out, domain.ErrValidation.WithDetails(map[string]any{
				"milestones": fmt.Sprintf("Le jalon « %s » doit tomber entre 0 et 3650 jours", m.Title),
			})
		}
		out.Milestones = append(out.Milestones, m)
	}

	out.Tasks = make([]TemplateTask, 0, len(in.Tasks))
	for i, t := range in.Tasks {
		t.Title = strings.TrimSpace(t.Title)
		t.Description = strings.TrimSpace(t.Description)
		if t.Title == "" {
			return out, domain.ErrValidation.WithDetails(map[string]any{
				"tasks": fmt.Sprintf("La tâche n° %d n'a pas de titre", i+1),
			})
		}
		if t.Priority == "" {
			t.Priority = "medium"
		}
		if _, ok := projectPriorities[t.Priority]; !ok {
			return out, domain.ErrValidation.WithDetails(map[string]any{
				"tasks": fmt.Sprintf("Priorité inconnue pour « %s »", t.Title),
			})
		}
		if t.OffsetDays != nil && (*t.OffsetDays < 0 || *t.OffsetDays > 3650) {
			return out, domain.ErrValidation.WithDetails(map[string]any{
				"tasks": fmt.Sprintf("La tâche « %s » doit tomber entre 0 et 3650 jours", t.Title),
			})
		}
		out.Tasks = append(out.Tasks, t)
	}

	return out, nil
}

// Create enregistre un nouveau modele et son contenu.
func (s *ProjectTemplateService) Create(ctx context.Context, in TemplateInput, createdBy uuid.UUID) (ProjectTemplate, error) {
	in, err := in.clean()
	if err != nil {
		return ProjectTemplate{}, err
	}

	return s.write(ctx, in, func(q *db.Queries) (uuid.UUID, error) {
		row, err := q.CreateProjectTemplate(ctx, db.CreateProjectTemplateParams{
			Name: in.Name, Description: in.Description, CreatedBy: &createdBy,
		})

		return row.ID, err
	})
}

// Update remplace un modele et tout son contenu.
func (s *ProjectTemplateService) Update(ctx context.Context, id uuid.UUID, in TemplateInput) (ProjectTemplate, error) {
	in, err := in.clean()
	if err != nil {
		return ProjectTemplate{}, err
	}

	return s.write(ctx, in, func(q *db.Queries) (uuid.UUID, error) {
		row, err := q.UpdateProjectTemplate(ctx, db.UpdateProjectTemplateParams{
			ID: id, Name: in.Name, Description: in.Description,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, domain.ErrNotFound
		}

		return row.ID, err
	})
}

// write ecrit l'en-tete d'un modele puis remplace son contenu, d'un bloc.
func (s *ProjectTemplateService) write(
	ctx context.Context,
	in TemplateInput,
	header func(q *db.Queries) (uuid.UUID, error),
) (ProjectTemplate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProjectTemplate{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	id, err := header(q)
	if err != nil {
		if isUniqueViolation(err) {
			return ProjectTemplate{}, domain.ErrValidation.WithDetails(map[string]any{
				"name": "Un modèle porte déjà ce nom",
			})
		}
		var domainErr *domain.Error
		if errors.As(err, &domainErr) {
			return ProjectTemplate{}, err
		}

		return ProjectTemplate{}, fmt.Errorf("ecriture du modele : %w", err)
	}

	if err := q.ClearTemplateContent(ctx, id); err != nil {
		return ProjectTemplate{}, fmt.Errorf("effacement du contenu : %w", err)
	}

	for _, serviceID := range in.ServiceIDs {
		if err := q.AddTemplateService(ctx, db.AddTemplateServiceParams{TemplateID: id, ServiceID: serviceID}); err != nil {
			if isForeignKeyViolation(err) {
				return ProjectTemplate{}, domain.ErrValidation.WithDetails(map[string]any{
					"service_ids": "Un des services n'existe pas",
				})
			}

			return ProjectTemplate{}, fmt.Errorf("services du modele : %w", err)
		}
	}

	for i, m := range in.Milestones {
		if err := q.AddTemplateMilestone(ctx, db.AddTemplateMilestoneParams{
			TemplateID: id, Title: m.Title, OffsetDays: int32(m.OffsetDays), Position: int32(i),
		}); err != nil {
			return ProjectTemplate{}, fmt.Errorf("jalons du modele : %w", err)
		}
	}

	for i, t := range in.Tasks {
		var offset *int32
		if t.OffsetDays != nil {
			v := int32(*t.OffsetDays)
			offset = &v
		}
		if err := q.AddTemplateTask(ctx, db.AddTemplateTaskParams{
			TemplateID: id, Title: t.Title, Description: t.Description,
			Priority: t.Priority, OffsetDays: offset, Position: int32(i),
		}); err != nil {
			return ProjectTemplate{}, fmt.Errorf("taches du modele : %w", err)
		}
	}

	out, err := s.get(ctx, q, id)
	if err != nil {
		return ProjectTemplate{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ProjectTemplate{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return out, nil
}

// Delete supprime un modele. Les projets crees a partir de lui ne bougent pas :
// ils en ont recu une copie, pas un lien.
func (s *ProjectTemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteProjectTemplate(ctx, id)
	if err != nil {
		return fmt.Errorf("suppression du modele : %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// applyTemplate copie un modele dans un projet qui vient d'etre cree, dans sa
// transaction. Les echeances partent du debut du projet, ou de sa creation
// quand il n'en a pas.
//
// Les services du modele s'ajoutent a ceux choisis dans le formulaire, sans
// doublon.
func applyTemplate(
	ctx context.Context,
	q *db.Queries,
	templateID, projectID uuid.UUID,
	start time.Time,
	createdBy uuid.UUID,
) error {
	if _, err := q.GetProjectTemplate(ctx, templateID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrValidation.WithDetails(map[string]any{"template_id": "Modèle introuvable"})
		}

		return fmt.Errorf("lecture du modele : %w", err)
	}

	services, err := q.ListTemplateServices(ctx, templateID)
	if err != nil {
		return fmt.Errorf("lecture des services du modele : %w", err)
	}
	for _, sv := range services {
		if err := q.AddProjectService(ctx, db.AddProjectServiceParams{ProjectID: projectID, ServiceID: sv.ID}); err != nil {
			return fmt.Errorf("services du modele : %w", err)
		}
	}

	milestones, err := q.ListTemplateMilestones(ctx, templateID)
	if err != nil {
		return fmt.Errorf("lecture des jalons du modele : %w", err)
	}
	for i, m := range milestones {
		due := start.AddDate(0, 0, int(m.OffsetDays))
		if err := q.CreateMilestoneFromTemplate(ctx, db.CreateMilestoneFromTemplateParams{
			ProjectID: projectID, Title: m.Title, DueOn: &due, Position: int32(i + 1), CreatedBy: &createdBy,
		}); err != nil {
			return fmt.Errorf("jalons du modele : %w", err)
		}
	}

	tasks, err := q.ListTemplateTasks(ctx, templateID)
	if err != nil {
		return fmt.Errorf("lecture des taches du modele : %w", err)
	}
	for _, t := range tasks {
		var due *time.Time
		if t.OffsetDays != nil {
			d := start.AddDate(0, 0, int(*t.OffsetDays))
			due = &d
		}
		if _, err := q.CreateTask(ctx, db.CreateTaskParams{
			ProjectID:   projectID,
			Title:       t.Title,
			Description: t.Description,
			Status:      "todo",
			Priority:    t.Priority,
			DueOn:       due,
			CreatedBy:   &createdBy,
		}); err != nil {
			return fmt.Errorf("taches du modele : %w", err)
		}
	}

	return nil
}
