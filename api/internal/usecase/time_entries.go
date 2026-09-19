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

// TimeEntry est une ligne de pointage.
type TimeEntry struct {
	ID uuid.UUID `json:"id"`
	// Jour pointe, au format AAAA-MM-JJ.
	SpentOn string `json:"spent_on"`
	// Duree en minutes : c'est l'unite de saisie, et la seule exacte. Les
	// heures se composent a l'affichage, sur des sommes.
	Minutes int32          `json:"minutes"`
	Note    string         `json:"note"`
	Project DeliverableRef `json:"project"`
	// Nulle pour un echange client, une reunion.
	Task *DeliverableRef `json:"task"`
	// Nul quand l'heure ne releve d'aucune prestation.
	Service *ServiceTag `json:"service"`
}

// TimeSheet est ce que l'ecran de saisie affiche : les lignes de la plage, leur
// total, et le total de chaque jour.
type TimeSheet struct {
	Items []TimeEntry `json:"items"`
	// Total de la plage, en minutes. Calcule par la base : la liste est bornee,
	// le total ne doit pas l'etre.
	TotalMinutes int64 `json:"total_minutes"`
	// Un poste par jour ayant recu du temps. Les jours vides n'y figurent pas :
	// c'est l'ecran qui connait la plage et sait completer.
	Days []TimeDay `json:"days"`
}

// TimeDay est le total d'un jour.
type TimeDay struct {
	Day     string `json:"day"`
	Minutes int64  `json:"minutes"`
}

// timeSheetLimit borne la liste. Une semaine de pointage tient largement
// dedans ; au-dela, c'est un rapport qu'il faut, pas un ecran de saisie.
const timeSheetLimit = 200

// TimeEntryService sert l'ecran « Saisie ».
type TimeEntryService struct {
	q *db.Queries
}

func NewTimeEntryService(pool *pgxpool.Pool) *TimeEntryService {
	return &TimeEntryService{q: db.New(pool)}
}

// TimeEntryInput est ce que le formulaire envoie.
type TimeEntryInput struct {
	ProjectID uuid.UUID
	TaskID    *uuid.UUID
	ServiceID *uuid.UUID
	SpentOn   time.Time
	Minutes   int32
	Note      string
}

// clean valide ce que le formulaire envoie.
//
// Les bornes doublent le CHECK de la base : la contrainte protege la donnee, ce
// controle-ci donne une erreur qui nomme le champ plutot qu'une violation
// remontee en 500.
func (in TimeEntryInput) clean() (TimeEntryInput, error) {
	out := in
	out.Note = strings.TrimSpace(in.Note)

	if out.Minutes <= 0 {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"minutes": "Une durée est attendue",
		})
	}

	if out.Minutes > 24*60 {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"minutes": "Une journée ne dépasse pas 24 heures",
		})
	}

	if out.SpentOn.IsZero() {
		return out, domain.ErrValidation.WithDetails(map[string]any{
			"spent_on": "La date est requise",
		})
	}

	return out, nil
}

// Sheet renvoie les saisies d'une personne sur une plage de jours.
func (s *TimeEntryService) Sheet(
	ctx context.Context,
	userID uuid.UUID,
	from, to time.Time,
) (TimeSheet, error) {
	if to.Before(from) {
		from, to = to, from
	}

	rows, err := s.q.ListTimeEntries(ctx, db.ListTimeEntriesParams{
		UserID:   userID,
		FromDay:  from,
		ToDay:    to,
		PageSize: timeSheetLimit,
	})
	if err != nil {
		return TimeSheet{}, fmt.Errorf("liste des saisies : %w", err)
	}

	total, err := s.q.SumTimeEntries(ctx, db.SumTimeEntriesParams{
		UserID: userID, FromDay: from, ToDay: to,
	})
	if err != nil {
		return TimeSheet{}, fmt.Errorf("total des saisies : %w", err)
	}

	perDay, err := s.q.SumTimeEntriesByDay(ctx, db.SumTimeEntriesByDayParams{
		UserID: userID, FromDay: from, ToDay: to,
	})
	if err != nil {
		return TimeSheet{}, fmt.Errorf("totaux par jour : %w", err)
	}

	sheet := TimeSheet{
		Items:        make([]TimeEntry, 0, len(rows)),
		TotalMinutes: total,
		Days:         make([]TimeDay, 0, len(perDay)),
	}

	for _, row := range rows {
		entry := TimeEntry{
			ID:      row.ID,
			SpentOn: row.SpentOn.Format(dateLayout),
			Minutes: row.Minutes,
			Note:    row.Note,
			Project: DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
		}

		if row.TaskID != nil && row.TaskTitle != nil {
			entry.Task = &DeliverableRef{ID: *row.TaskID, Name: *row.TaskTitle}
		}

		if row.ServiceID != nil && row.ServiceName != nil {
			tag := ServiceTag{ID: *row.ServiceID, Name: *row.ServiceName}
			if row.ServiceColor != nil {
				tag.Color = *row.ServiceColor
			}

			entry.Service = &tag
		}

		sheet.Items = append(sheet.Items, entry)
	}

	for _, day := range perDay {
		sheet.Days = append(sheet.Days, TimeDay{
			Day:     day.SpentOn.Format(dateLayout),
			Minutes: day.Minutes,
		})
	}

	return sheet, nil
}

// Create enregistre une saisie.
func (s *TimeEntryService) Create(
	ctx context.Context,
	userID uuid.UUID,
	in TimeEntryInput,
) (TimeEntry, error) {
	clean, err := in.clean()
	if err != nil {
		return TimeEntry{}, err
	}

	id, err := s.q.CreateTimeEntry(ctx, db.CreateTimeEntryParams{
		UserID:    userID,
		ProjectID: clean.ProjectID,
		TaskID:    clean.TaskID,
		ServiceID: clean.ServiceID,
		SpentOn:   clean.SpentOn,
		Minutes:   clean.Minutes,
		Note:      clean.Note,
	})
	if err != nil {
		// Un projet, une tache ou un service inconnu est une faute de la
		// requete : la cle etrangere le dit, on le rend en 422.
		if isForeignKeyViolation(err) {
			return TimeEntry{}, domain.ErrValidation.WithDetails(map[string]any{
				"project_id": "Ce projet, cette tâche ou ce service n'existe pas",
			})
		}

		return TimeEntry{}, fmt.Errorf("creation de la saisie : %w", err)
	}

	return s.Get(ctx, userID, id)
}

// Update modifie une saisie.
//
// L'identifiant du compte entre dans la clause : une saisie qui n'est pas la
// sienne ne correspond a aucune ligne, et l'appelant lit un 404 plutot que de
// voir qu'elle existe.
func (s *TimeEntryService) Update(
	ctx context.Context,
	userID, id uuid.UUID,
	in TimeEntryInput,
) (TimeEntry, error) {
	clean, err := in.clean()
	if err != nil {
		return TimeEntry{}, err
	}

	if _, err := s.q.UpdateTimeEntry(ctx, db.UpdateTimeEntryParams{
		ID:        id,
		UserID:    userID,
		ProjectID: clean.ProjectID,
		TaskID:    clean.TaskID,
		ServiceID: clean.ServiceID,
		SpentOn:   clean.SpentOn,
		Minutes:   clean.Minutes,
		Note:      clean.Note,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TimeEntry{}, domain.ErrNotFound
		}
		if isForeignKeyViolation(err) {
			return TimeEntry{}, domain.ErrValidation.WithDetails(map[string]any{
				"project_id": "Ce projet, cette tâche ou ce service n'existe pas",
			})
		}

		return TimeEntry{}, fmt.Errorf("mise a jour de la saisie : %w", err)
	}

	return s.Get(ctx, userID, id)
}

// Delete retire une saisie.
func (s *TimeEntryService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	rows, err := s.q.DeleteTimeEntry(ctx, db.DeleteTimeEntryParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("suppression de la saisie : %w", err)
	}

	if rows == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// Get renvoie une saisie.
func (s *TimeEntryService) Get(
	ctx context.Context,
	userID, id uuid.UUID,
) (TimeEntry, error) {
	row, err := s.q.GetTimeEntry(ctx, db.GetTimeEntryParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TimeEntry{}, domain.ErrNotFound
		}

		return TimeEntry{}, fmt.Errorf("lecture de la saisie : %w", err)
	}

	entry := TimeEntry{
		ID:      row.ID,
		SpentOn: row.SpentOn.Format(dateLayout),
		Minutes: row.Minutes,
		Note:    row.Note,
		Project: DeliverableRef{ID: row.ProjectID, Name: row.ProjectName},
	}

	if row.TaskID != nil && row.TaskTitle != nil {
		entry.Task = &DeliverableRef{ID: *row.TaskID, Name: *row.TaskTitle}
	}

	if row.ServiceID != nil && row.ServiceName != nil {
		tag := ServiceTag{ID: *row.ServiceID, Name: *row.ServiceName}
		if row.ServiceColor != nil {
			tag.Color = *row.ServiceColor
		}

		entry.Service = &tag
	}

	return entry, nil
}
