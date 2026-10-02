package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/updates"
)

// Erreurs de la mise a jour. Des 409 : la demande est valide, c'est l'etat de
// l'instance qui ne la permet pas.
var (
	ErrUpdaterUnavailable = &domain.Error{
		Status: http.StatusConflict, Code: "UPDATER_UNAVAILABLE",
		Message: "Le service de mise à jour ne répond pas sur cette instance",
	}
	ErrUpdateNotAvailable = &domain.Error{
		Status: http.StatusConflict, Code: "UPDATE_NOT_AVAILABLE",
		Message: "Aucune nouvelle version à installer",
	}
	ErrUpdateInProgress = &domain.Error{
		Status: http.StatusConflict, Code: "UPDATE_IN_PROGRESS",
		Message: "Une mise à jour est déjà en cours",
	}
)

// updaterSilence : au-dela de ce delai sans signe de vie, l'updater est juge
// absent. Il en donne un toutes les trente secondes.
const updaterSilence = 2 * time.Minute

// ReleaseInfo est une version publiee.
type ReleaseInfo struct {
	Version     string     `json:"version"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	PublishedAt *time.Time `json:"published_at"`
}

// UpdateRequestInfo est la derniere demande de mise a jour.
type UpdateRequestInfo struct {
	ID            uuid.UUID  `json:"id"`
	Status        string     `json:"status"`
	FromVersion   string     `json:"from_version"`
	TargetVersion string     `json:"target_version"`
	RequestedBy   string     `json:"requested_by"`
	Step          string     `json:"step"`
	Error         string     `json:"error"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at"`
}

// UpdateStatus est ce que l'ecran de mise a jour affiche.
type UpdateStatus struct {
	CurrentVersion string `json:"current_version"`
	// Derniere version publiee, nulle tant qu'aucune verification n'a abouti.
	Latest          *ReleaseInfo `json:"latest"`
	UpdateAvailable bool         `json:"update_available"`
	// L'updater donne signe de vie et a trouve l'application : le bouton peut
	// installer la mise a jour.
	CanUpdate bool `json:"can_update"`
	// Ce qui empeche l'updater de travailler, vide quand il est pret ou
	// qu'il n'a jamais donne signe de vie.
	UpdaterError string     `json:"updater_error"`
	CheckEnabled bool       `json:"check_enabled"`
	CheckedAt    *time.Time `json:"checked_at"`
	// Erreur du dernier passage de verification, vide quand il a abouti.
	CheckError  string             `json:"check_error"`
	LastRequest *UpdateRequestInfo `json:"last_request"`
	// Une mise a jour est en route : le bouton est remplace par son suivi.
	InProgress bool `json:"in_progress"`
}

// UpdateService porte la mise a jour de l'application, cote API : il lit ce
// que la verification des versions et l'updater ont ecrit, et enregistre les
// demandes. L'updater, lui, les execute.
type UpdateService struct {
	q            *db.Queries
	current      string
	checkEnabled bool
}

// NewUpdateService construit le service. `current` est la version du binaire
// en cours d'execution.
func NewUpdateService(pool *pgxpool.Pool, current string, checkEnabled bool) *UpdateService {
	return &UpdateService{q: db.New(pool), current: current, checkEnabled: checkEnabled}
}

// Status lit l'etat de la mise a jour : trois requetes sur des tables d'une
// poignee de lignes.
func (s *UpdateService) Status(ctx context.Context) (UpdateStatus, error) {
	return s.statusAt(ctx, time.Now())
}

func (s *UpdateService) statusAt(ctx context.Context, now time.Time) (UpdateStatus, error) {
	status := UpdateStatus{CurrentVersion: s.current, CheckEnabled: s.checkEnabled}

	check, err := s.q.GetReleaseCheck(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Aucun passage encore : l'instance vient de demarrer.
	case err != nil:
		return UpdateStatus{}, fmt.Errorf("lecture de la derniere version : %w", err)
	default:
		checkedAt := check.CheckedAt
		status.CheckedAt = &checkedAt
		status.CheckError = check.Error
		if check.Version != "" {
			status.Latest = &ReleaseInfo{
				Version: check.Version, Name: check.Name, URL: check.Url, PublishedAt: check.PublishedAt,
			}
			status.UpdateAvailable = updates.Newer(check.Version, s.current)
		}
	}

	updater, err := s.q.GetUpdater(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Jamais vu : pas d'updater dans cette installation.
	case err != nil:
		return UpdateStatus{}, fmt.Errorf("lecture de l'updater : %w", err)
	default:
		alive := now.Sub(updater.LastSeen) < updaterSilence
		status.CanUpdate = alive && updater.Error == ""
		if alive {
			status.UpdaterError = updater.Error
		}
	}

	last, err := s.q.GetLatestUpdateRequest(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return UpdateStatus{}, fmt.Errorf("lecture de la derniere demande : %w", err)
	default:
		status.LastRequest = &UpdateRequestInfo{
			ID:            last.ID,
			Status:        last.Status,
			FromVersion:   last.FromVersion,
			TargetVersion: last.TargetVersion,
			RequestedBy:   last.RequestedByName,
			Step:          last.Step,
			Error:         last.Error,
			CreatedAt:     last.CreatedAt,
			FinishedAt:    last.FinishedAt,
		}
		status.InProgress = last.Status == "pending" || last.Status == "running"
	}

	return status, nil
}

// Request enregistre une demande de mise a jour vers la derniere version.
// L'updater la prend dans les secondes qui suivent.
func (s *UpdateService) Request(ctx context.Context, actor uuid.UUID) (UpdateStatus, error) {
	status, err := s.Status(ctx)
	if err != nil {
		return UpdateStatus{}, err
	}

	switch {
	case status.InProgress:
		return UpdateStatus{}, ErrUpdateInProgress
	case !status.UpdateAvailable:
		return UpdateStatus{}, ErrUpdateNotAvailable
	case !status.CanUpdate:
		return UpdateStatus{}, ErrUpdaterUnavailable
	}

	if _, err := s.q.CreateUpdateRequest(ctx, db.CreateUpdateRequestParams{
		RequestedBy:   &actor,
		FromVersion:   s.current,
		TargetVersion: status.Latest.Version,
	}); err != nil {
		// Deux admins au meme instant : l'index unique n'en laisse passer
		// qu'un.
		if isUniqueViolation(err) {
			return UpdateStatus{}, ErrUpdateInProgress
		}
		return UpdateStatus{}, fmt.Errorf("enregistrement de la demande : %w", err)
	}

	return s.Status(ctx)
}
