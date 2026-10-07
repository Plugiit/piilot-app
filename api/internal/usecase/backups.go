package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// BackupInfo est une sauvegarde du journal.
type BackupInfo struct {
	ID         uuid.UUID  `json:"id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	// running, done ou failed.
	Status    string `json:"status"`
	Location  string `json:"location"`
	SizeBytes int64  `json:"size_bytes"`
	Error     string `json:"error"`
}

// BackupStatus est l'ecran Paramètres > Sauvegardes.
type BackupStatus struct {
	// La derniere reussie, nulle tant qu'il n'y en a aucune.
	LastSuccess *BackupInfo `json:"last_success"`
	// Au-dela de ce delai sans sauvegarde reussie, l'instance est jugee sans
	// sauvegarde : l'ecran le dit en rouge, les admins sont prevenus.
	StaleAfter string `json:"stale_after"`
	// Vrai quand la derniere reussie date de plus de StaleAfter, ou qu'il n'y
	// en a jamais eu.
	Stale bool `json:"stale"`
	// Le journal, les plus recentes d'abord.
	Items []BackupInfo `json:"items"`
}

// BackupService lit le journal des sauvegardes. La commande `backup`, un
// conteneur a part, l'ecrit.
type BackupService struct {
	q          *db.Queries
	staleAfter time.Duration
}

// NewBackupService construit le service.
func NewBackupService(pool *pgxpool.Pool, staleAfter time.Duration) *BackupService {
	return &BackupService{q: db.New(pool), staleAfter: staleAfter}
}

// Status lit le journal : deux requetes sur une table d'une poignee de lignes.
func (s *BackupService) Status(ctx context.Context) (BackupStatus, error) {
	return s.statusAt(ctx, time.Now())
}

func (s *BackupService) statusAt(ctx context.Context, now time.Time) (BackupStatus, error) {
	status := BackupStatus{StaleAfter: s.staleAfter.String(), Stale: true, Items: []BackupInfo{}}

	last, err := s.q.GetLastSuccessfulBackup(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return BackupStatus{}, fmt.Errorf("derniere sauvegarde : %w", err)
	default:
		info := backupOf(last)
		status.LastSuccess = &info
		status.Stale = s.staleAfter > 0 && now.Sub(last.StartedAt) > s.staleAfter
	}

	rows, err := s.q.ListBackups(ctx)
	if err != nil {
		return BackupStatus{}, fmt.Errorf("journal des sauvegardes : %w", err)
	}
	for _, row := range rows {
		status.Items = append(status.Items, backupOf(row))
	}

	return status, nil
}

func backupOf(row db.AppBackup) BackupInfo {
	return BackupInfo{
		ID: row.ID, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt,
		Status: row.Status, Location: row.Location, SizeBytes: row.SizeBytes, Error: row.Error,
	}
}
