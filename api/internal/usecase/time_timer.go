package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Timer est le chrono d'une personne, tel que l'ecran le montre.
type Timer struct {
	Project   NamedRef  `json:"project"`
	Task      *NamedRef `json:"task"`
	Note      string    `json:"note"`
	StartedAt time.Time `json:"started_at"`
}

// NamedRef designe une chose par son identifiant et son nom.
type NamedRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// TimerInput est ce qu'on demarre.
type TimerInput struct {
	ProjectID uuid.UUID
	TaskID    *uuid.UUID
	Note      string
}

// TimerService demarre et arrete le chrono. A l'arret, il ecrit la saisie de
// temps par le service des saisies : memes regles, memes compteurs.
type TimerService struct {
	q       *db.Queries
	entries *TimeEntryService
	now     func() time.Time
}

func NewTimerService(pool *pgxpool.Pool, entries *TimeEntryService) *TimerService {
	return &TimerService{q: db.New(pool), entries: entries, now: time.Now}
}

// Current rend le chrono en cours, ou nil.
func (s *TimerService) Current(ctx context.Context, userID uuid.UUID) (*Timer, error) {
	row, err := s.q.GetTimer(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lecture du chrono : %w", err)
	}

	return timerOf(row), nil
}

func timerOf(row db.GetTimerRow) *Timer {
	t := &Timer{
		Project:   NamedRef{ID: row.ProjectID, Name: row.ProjectName},
		Note:      row.Note,
		StartedAt: row.StartedAt,
	}
	if row.TaskID != nil && row.TaskTitle != nil {
		t.Task = &NamedRef{ID: *row.TaskID, Name: *row.TaskTitle}
	}
	return t
}

// Start lance le chrono. S'il en tourne deja un, il est d'abord arrete et
// son temps enregistre : passer d'une tache a l'autre est un seul geste.
func (s *TimerService) Start(ctx context.Context, userID uuid.UUID, in TimerInput) (*Timer, error) {
	if _, err := s.Stop(ctx, userID); err != nil {
		return nil, err
	}

	if err := s.q.StartTimer(ctx, db.StartTimerParams{
		UserID: userID, ProjectID: in.ProjectID, TaskID: in.TaskID, Note: in.Note,
	}); err != nil {
		if isForeignKeyViolation(err) {
			return nil, domain.ErrValidation.WithDetails(map[string]any{"project_id": "Projet ou tâche introuvable"})
		}
		return nil, fmt.Errorf("demarrage du chrono : %w", err)
	}

	return s.Current(ctx, userID)
}

// Stop arrete le chrono et ecrit la saisie correspondante. Rend nil, nil quand
// rien ne tournait. Le temps est arrondi a la minute superieure, une minute au
// moins : un chrono lance par erreur et arrete aussitot se corrige dans la
// feuille, il ne disparait pas en silence.
func (s *TimerService) Stop(ctx context.Context, userID uuid.UUID) (*TimeEntry, error) {
	row, err := s.q.GetTimer(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lecture du chrono : %w", err)
	}

	elapsed := s.now().Sub(row.StartedAt).Minutes()
	minutes := int32(math.Ceil(elapsed))
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 1440 {
		minutes = 1440
	}

	entry, err := s.entries.Create(ctx, userID, TimeEntryInput{
		ProjectID: row.ProjectID,
		TaskID:    row.TaskID,
		SpentOn:   agencyToday(row.StartedAt),
		Minutes:   minutes,
		Note:      row.Note,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.q.DeleteTimer(ctx, userID); err != nil {
		return nil, fmt.Errorf("arret du chrono : %w", err)
	}

	return &entry, nil
}

// Discard abandonne le chrono sans rien ecrire.
func (s *TimerService) Discard(ctx context.Context, userID uuid.UUID) error {
	if _, err := s.q.DeleteTimer(ctx, userID); err != nil {
		return fmt.Errorf("abandon du chrono : %w", err)
	}
	return nil
}
