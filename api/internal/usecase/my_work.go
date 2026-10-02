package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Bornes de chaque bloc de « Mon travail ». La page dit ce qui presse, elle ne
// remplace pas les listes completes : au-dela, chaque bloc renvoie vers son
// ecran, et le total annonce ce qui reste.
const (
	myWorkTasks        = 30
	myWorkTickets      = 10
	myWorkDeliverables = 10
	myWorkProjects     = 24
)

// agencyZone : « aujourd'hui » et « cette semaine » sont ceux de l'agence, quel
// que soit le fuseau du serveur — un conteneur tourne volontiers en UTC, et une
// tache due ce jour passerait en retard des 1 h du matin.
var agencyZone = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// WorkTask est une tache de la page.
type WorkTask struct {
	ID       uuid.UUID      `json:"id"`
	Title    string         `json:"title"`
	Status   string         `json:"status"`
	Priority string         `json:"priority"`
	DueOn    *string        `json:"due_on"`
	Overdue  bool           `json:"overdue"`
	Project  DeliverableRef `json:"project"`
}

// WorkTicket est un ticket confie a la personne.
type WorkTicket struct {
	ID        uuid.UUID      `json:"id"`
	Numero    int64          `json:"numero"`
	Subject   string         `json:"subject"`
	Status    string         `json:"status"`
	Priority  string         `json:"priority"`
	UpdatedAt time.Time      `json:"updated_at"`
	Project   DeliverableRef `json:"project"`
}

// WorkDeliverable est un livrable a deposer, ou a reprendre.
type WorkDeliverable struct {
	ID      uuid.UUID      `json:"id"`
	Title   string         `json:"title"`
	Project DeliverableRef `json:"project"`
	// "draft" : rien n'a encore ete depose. "feedback" : le client a renvoye
	// des retours sur la version courante.
	State     string     `json:"state"`
	Version   *int32     `json:"version"`
	DecidedAt *time.Time `json:"decided_at"`
	Feedback  string     `json:"feedback"`
}

// WorkProject est un projet ou la personne intervient.
type WorkProject struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ClientName string    `json:"client_name"`
	Status     string    `json:"status"`
	Progress   int       `json:"progress"`
	DueOn      *string   `json:"due_on"`
	TasksTotal int       `json:"tasks_total"`
	TasksDone  int       `json:"tasks_done"`
}

// MyWork est tout ce qu'affiche la page « Mon travail », en un appel.
type MyWork struct {
	Tasks []WorkTask `json:"tasks"`
	// Comptes sur toutes les taches ouvertes, pas seulement celles rendues.
	TasksTotal   int64 `json:"tasks_total"`
	TasksOverdue int64 `json:"tasks_overdue"`

	Tickets      []WorkTicket `json:"tickets"`
	TicketsTotal int64        `json:"tickets_total"`

	Deliverables      []WorkDeliverable `json:"deliverables"`
	DeliverablesTotal int64             `json:"deliverables_total"`

	Projects []WorkProject `json:"projects"`

	// Semaine en cours : son lundi, et le temps deja saisi.
	WeekStart   string `json:"week_start"`
	WeekMinutes int64  `json:"week_minutes"`
	Today       string `json:"today"`
}

// MyWorkService sert la page « Mon travail ».
type MyWorkService struct {
	q *db.Queries
}

func NewMyWorkService(pool *pgxpool.Pool) *MyWorkService {
	return &MyWorkService{q: db.New(pool)}
}

// Get rend le travail d'une personne. L'identifiant vient de la session.
func (s *MyWorkService) Get(ctx context.Context, userID uuid.UUID) (MyWork, error) {
	return s.at(ctx, userID, time.Now())
}

func (s *MyWorkService) at(ctx context.Context, userID uuid.UUID, now time.Time) (MyWork, error) {
	local := now.In(agencyZone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	week := weekStartOf(today)

	work := MyWork{
		Tasks:        []WorkTask{},
		Tickets:      []WorkTicket{},
		Deliverables: []WorkDeliverable{},
		Projects:     []WorkProject{},
		WeekStart:    week.Format(time.DateOnly),
		Today:        today.Format(time.DateOnly),
	}

	tasks, err := s.q.ListMyOpenTasks(ctx, db.ListMyOpenTasksParams{
		Today: today, UserID: userID, PageSize: myWorkTasks,
	})
	if err != nil {
		return MyWork{}, fmt.Errorf("lecture des taches : %w", err)
	}
	for _, row := range tasks {
		work.TasksTotal, work.TasksOverdue = row.Total, row.Overdue
		work.Tasks = append(work.Tasks, WorkTask{
			ID:       row.ID,
			Title:    row.Title,
			Status:   row.Status,
			Priority: row.Priority,
			DueOn:    formatDate(row.DueOn),
			Overdue:  row.DueOn != nil && row.DueOn.Before(today),
			Project:  DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
		})
	}

	tickets, err := s.q.ListMyOpenTickets(ctx, db.ListMyOpenTicketsParams{
		UserID: &userID, PageSize: myWorkTickets,
	})
	if err != nil {
		return MyWork{}, fmt.Errorf("lecture des tickets : %w", err)
	}
	for _, row := range tickets {
		work.TicketsTotal = row.Total
		work.Tickets = append(work.Tickets, WorkTicket{
			ID:        row.ID,
			Numero:    row.Numero,
			Subject:   row.Subject,
			Status:    row.Status,
			Priority:  row.Priority,
			UpdatedAt: row.UpdatedAt,
			Project:   DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
		})
	}

	deliverables, err := s.q.ListMyDeliverablesToSubmit(ctx, db.ListMyDeliverablesToSubmitParams{
		UserID: userID, PageSize: myWorkDeliverables,
	})
	if err != nil {
		return MyWork{}, fmt.Errorf("lecture des livrables : %w", err)
	}
	for _, row := range deliverables {
		work.DeliverablesTotal = row.Total
		state := "draft"
		if row.Version != nil {
			state = "feedback"
		}
		work.Deliverables = append(work.Deliverables, WorkDeliverable{
			ID:        row.ID,
			Title:     row.Title,
			Project:   DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
			State:     state,
			Version:   row.Version,
			DecidedAt: row.DecidedAt,
			Feedback:  row.Feedback,
		})
	}

	projects, err := s.q.ListMyProjects(ctx, db.ListMyProjectsParams{
		UserID: userID, PageSize: myWorkProjects,
	})
	if err != nil {
		return MyWork{}, fmt.Errorf("lecture des projets : %w", err)
	}
	for _, row := range projects {
		work.Projects = append(work.Projects, WorkProject{
			ID:         row.ID,
			Name:       row.Name,
			ClientName: row.ClientName,
			Status:     row.Status,
			Progress:   int(row.Progress),
			DueOn:      formatDate(row.DueOn),
			TasksTotal: int(row.TasksTotal),
			TasksDone:  int(row.TasksDone),
		})
	}

	minutes, err := s.q.SumMyWeekMinutes(ctx, db.SumMyWeekMinutesParams{UserID: userID, WeekStart: week})
	if err != nil {
		return MyWork{}, fmt.Errorf("lecture du temps de la semaine : %w", err)
	}
	work.WeekMinutes = minutes

	return work, nil
}
