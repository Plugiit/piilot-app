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

// MilestoneDeliverable est un livrable tel que la liste des jalons le montre.
type MilestoneDeliverable struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
	// brouillon, en_attente, valide ou retours.
	Status string `json:"status"`
}

// Milestone est un jalon et ses livrables.
type Milestone struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueOn       *string    `json:"due_on"`
	CompletedAt *time.Time `json:"completed_at"`
	// "done" si atteint, "late" si l'echeance est passee sans qu'il le soit,
	// "upcoming" sinon. Calcule ici pour que la liste et le planning disent la
	// meme chose.
	State                 string                 `json:"state"`
	DeliverablesTotal     int                    `json:"deliverables_total"`
	DeliverablesValidated int                    `json:"deliverables_validated"`
	Deliverables          []MilestoneDeliverable `json:"deliverables"`
}

// MilestoneList est l'onglet « Jalons » d'un projet.
type MilestoneList struct {
	Items []Milestone `json:"items"`
}

// MilestoneInput porte la creation ou la modification d'un jalon. Pour une
// modification, un champ nul garde sa valeur.
type MilestoneInput struct {
	Title       *string
	Description *string
	SetDue      bool
	DueOn       *time.Time
	SetDone     bool
	Done        bool
}

// PlanningItem est une ligne du calendrier.
type PlanningItem struct {
	// milestone, project_due ou task_due.
	Kind    string         `json:"kind"`
	ID      uuid.UUID      `json:"id"`
	Title   string         `json:"title"`
	Day     string         `json:"day"`
	Done    bool           `json:"done"`
	Project DeliverableRef `json:"project"`
	// Pour un jalon : ses livrables, et combien sont valides.
	DeliverablesTotal     int `json:"deliverables_total"`
	DeliverablesValidated int `json:"deliverables_validated"`
}

// Planning est le calendrier d'une periode.
type Planning struct {
	From  string         `json:"from"`
	To    string         `json:"to"`
	Items []PlanningItem `json:"items"`
}

// planningMaxDays borne la periode demandee : un mois affiche, avec ses
// semaines a cheval, tient en six semaines.
const planningMaxDays = 62

// MilestoneService sert les jalons et le planning.
type MilestoneService struct {
	q *db.Queries
}

func NewMilestoneService(pool *pgxpool.Pool) *MilestoneService {
	return &MilestoneService{q: db.New(pool)}
}

// agencyToday rend la date du jour de l'agence, a minuit UTC, comme les
// colonnes date.
func agencyToday(now time.Time) time.Time {
	local := now.In(agencyZone)

	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func milestoneState(row db.Milestone, today time.Time) string {
	switch {
	case row.CompletedAt != nil:
		return "done"
	case row.DueOn != nil && row.DueOn.Before(today):
		return "late"
	default:
		return "upcoming"
	}
}

func milestoneOf(row db.Milestone, today time.Time) Milestone {
	return Milestone{
		ID:                    row.ID,
		Title:                 row.Title,
		Description:           row.Description,
		DueOn:                 formatDate(row.DueOn),
		CompletedAt:           row.CompletedAt,
		State:                 milestoneState(row, today),
		DeliverablesTotal:     int(row.DeliverablesTotal),
		DeliverablesValidated: int(row.DeliverablesValidated),
		Deliverables:          []MilestoneDeliverable{},
	}
}

// List rend les jalons d'un projet avec leurs livrables : deux requetes, quel
// que soit le nombre de jalons.
func (s *MilestoneService) List(ctx context.Context, projectID uuid.UUID) (MilestoneList, error) {
	rows, err := s.q.ListProjectMilestones(ctx, projectID)
	if err != nil {
		return MilestoneList{}, fmt.Errorf("lecture des jalons : %w", err)
	}

	deliverables, err := s.q.ListMilestoneDeliverables(ctx, projectID)
	if err != nil {
		return MilestoneList{}, fmt.Errorf("lecture des livrables des jalons : %w", err)
	}

	today := agencyToday(time.Now())
	items := make([]Milestone, 0, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, row := range rows {
		items = append(items, milestoneOf(row, today))
		index[row.ID] = i
	}

	for _, d := range deliverables {
		if d.MilestoneID == nil {
			continue
		}
		if i, ok := index[*d.MilestoneID]; ok {
			items[i].Deliverables = append(items[i].Deliverables, MilestoneDeliverable{
				ID: d.ID, Title: d.Title, Status: d.Status,
			})
		}
	}

	return MilestoneList{Items: items}, nil
}

// Create ajoute un jalon a un projet.
func (s *MilestoneService) Create(
	ctx context.Context,
	projectID uuid.UUID,
	in MilestoneInput,
	createdBy uuid.UUID,
) (Milestone, error) {
	title := ""
	if in.Title != nil {
		title = strings.TrimSpace(*in.Title)
	}
	if title == "" {
		return Milestone{}, domain.ErrValidation.WithDetails(map[string]any{"title": "Le titre est requis"})
	}

	description := ""
	if in.Description != nil {
		description = strings.TrimSpace(*in.Description)
	}

	row, err := s.q.CreateMilestone(ctx, db.CreateMilestoneParams{
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		DueOn:       in.DueOn,
		CreatedBy:   &createdBy,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return Milestone{}, domain.ErrNotFound
		}

		return Milestone{}, fmt.Errorf("creation du jalon : %w", err)
	}

	return milestoneOf(row, agencyToday(time.Now())), nil
}

// Update modifie un jalon, ou le marque atteint.
func (s *MilestoneService) Update(ctx context.Context, id uuid.UUID, in MilestoneInput) (Milestone, error) {
	if in.Title != nil {
		trimmed := strings.TrimSpace(*in.Title)
		if trimmed == "" {
			return Milestone{}, domain.ErrValidation.WithDetails(map[string]any{"title": "Le titre est requis"})
		}
		in.Title = &trimmed
	}
	if in.Description != nil {
		trimmed := strings.TrimSpace(*in.Description)
		in.Description = &trimmed
	}

	row, err := s.q.UpdateMilestone(ctx, db.UpdateMilestoneParams{
		ID:           id,
		Title:        in.Title,
		Description:  in.Description,
		SetDue:       in.SetDue,
		DueOn:        in.DueOn,
		SetCompleted: in.SetDone,
		Completed:    in.Done,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Milestone{}, domain.ErrNotFound
		}

		return Milestone{}, fmt.Errorf("modification du jalon : %w", err)
	}

	return milestoneOf(row, agencyToday(time.Now())), nil
}

// Delete supprime un jalon. Ses livrables restent, detaches.
func (s *MilestoneService) Delete(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteMilestone(ctx, id)
	if err != nil {
		return fmt.Errorf("suppression du jalon : %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// AttachDeliverable rattache un livrable a un jalon de son projet, ou le
// detache quand `milestoneID` est nul.
func (s *MilestoneService) AttachDeliverable(ctx context.Context, deliverableID uuid.UUID, milestoneID *uuid.UUID) error {
	n, err := s.q.SetDeliverableMilestone(ctx, db.SetDeliverableMilestoneParams{
		ID:          deliverableID,
		MilestoneID: milestoneID,
	})
	if err != nil {
		return fmt.Errorf("rattachement du livrable : %w", err)
	}

	// Rien de touche : le livrable n'existe pas, ou le jalon n'est pas de son
	// projet. Les deux disent la meme chose a l'appelant.
	if n == 0 {
		return domain.ErrValidation.WithDetails(map[string]any{
			"milestone_id": "Ce jalon n'appartient pas au projet du livrable",
		})
	}

	return nil
}

// Planning rend le calendrier d'une periode d'au plus planningMaxDays jours.
func (s *MilestoneService) Planning(
	ctx context.Context,
	userID uuid.UUID,
	from, to time.Time,
	mine bool,
) (Planning, error) {
	if to.Before(from) {
		return Planning{}, domain.ErrValidation.WithDetails(map[string]any{"to": "La fin précède le début"})
	}
	if to.Sub(from) > planningMaxDays*24*time.Hour {
		return Planning{}, domain.ErrValidation.WithDetails(map[string]any{
			"to": fmt.Sprintf("Une période ne dépasse pas %d jours", planningMaxDays),
		})
	}

	rows, err := s.q.ListPlanning(ctx, db.ListPlanningParams{
		FromDay: from, ToDay: to, Mine: mine, UserID: userID,
	})
	if err != nil {
		return Planning{}, fmt.Errorf("lecture du planning : %w", err)
	}

	items := make([]PlanningItem, 0, len(rows))
	for _, row := range rows {
		day := formatDate(row.Day)
		if day == nil {
			continue
		}
		items = append(items, PlanningItem{
			Kind:                  row.Kind,
			ID:                    row.ID,
			Title:                 row.Title,
			Day:                   *day,
			Done:                  row.Done,
			Project:               DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
			DeliverablesTotal:     int(row.DeliverablesTotal),
			DeliverablesValidated: int(row.DeliverablesValidated),
		})
	}

	return Planning{
		From:  from.Format(time.DateOnly),
		To:    to.Format(time.DateOnly),
		Items: items,
	}, nil
}
