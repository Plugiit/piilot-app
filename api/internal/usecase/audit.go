package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Journal d'audit : qui a fait quoi, quand, d'ou.
//
// Les lignes s'ecrivent depuis un intercepteur HTTP qui connait l'appelant,
// son adresse et la route ; la table refuse toute modification et ne
// s'efface que par la purge de retention.

// AuditEntry est une ligne a ecrire.
type AuditEntry struct {
	ActorID    *uuid.UUID
	Action     string
	TargetType string
	TargetID   string
	IP         string
	UserAgent  string
	Details    map[string]any
}

// AuditItem est une ligne du journal.
type AuditItem struct {
	ID         uuid.UUID      `json:"id"`
	At         time.Time      `json:"at"`
	ActorID    *uuid.UUID     `json:"actor_id"`
	ActorEmail string         `json:"actor_email"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	IP         string         `json:"ip"`
	UserAgent  string         `json:"user_agent"`
	Details    map[string]any `json:"details"`
}

// AuditPage est une page du journal.
type AuditPage struct {
	Items []AuditItem `json:"items"`
	Total int         `json:"total"`
	Page  int         `json:"page"`
	// Retention, en jours, pour l'ecran.
	RetentionDays int `json:"retention_days"`
}

// AuditFilters porte les filtres de l'ecran.
type AuditFilters struct {
	Action  string
	ActorID *uuid.UUID
	Since   *time.Time
	Until   *time.Time
	Search  string
}

// AuditService ecrit et lit le journal.
type AuditService struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	retention time.Duration
}

// NewAuditService construit le service.
func NewAuditService(pool *pgxpool.Pool, retention time.Duration) *AuditService {
	return &AuditService{pool: pool, q: db.New(pool), retention: retention}
}

// Record ecrit une ligne. L'adresse de l'acteur est figee au moment du geste.
func (s *AuditService) Record(ctx context.Context, e AuditEntry) error {
	email := ""
	if e.ActorID != nil {
		if u, err := s.q.GetUserByID(ctx, *e.ActorID); err == nil {
			email = u.Email
		}
	}
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	details, err := json.Marshal(e.Details)
	if err != nil {
		return err
	}
	return s.q.InsertAudit(ctx, db.InsertAuditParams{
		ActorID: e.ActorID, ActorEmail: email, Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID,
		Ip: e.IP, UserAgent: truncate(e.UserAgent, 300), Details: details,
	})
}

const auditPageSize = 50

// AuditExportMax borne un export : au-dela, on resserre les dates.
const AuditExportMax = 50000

func (f AuditFilters) params() (action, search *string) {
	// Un prefixe (« auth. ») ou un suffixe (« *.deleted ») : l'etoile tient
	// lieu de joker, le reste est un debut d'action.
	if a := strings.TrimSpace(f.Action); a != "" {
		pattern := strings.ReplaceAll(a, "*", "%")
		if !strings.HasSuffix(pattern, "%") {
			pattern += "%"
		}
		action = &pattern
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		search = &q
	}
	return action, search
}

// List rend une page du journal.
func (s *AuditService) List(ctx context.Context, f AuditFilters, page int) (AuditPage, error) {
	if page < 1 {
		page = 1
	}
	rows, total, err := s.read(ctx, f, auditPageSize, (page-1)*auditPageSize)
	if err != nil {
		return AuditPage{}, err
	}
	return AuditPage{Items: rows, Total: total, Page: page, RetentionDays: int(s.retention.Hours() / 24)}, nil
}

// Export rend les lignes d'un filtre, AuditExportMax au plus.
func (s *AuditService) Export(ctx context.Context, f AuditFilters) ([]AuditItem, int, error) {
	return s.read(ctx, f, AuditExportMax, 0)
}

func (s *AuditService) read(ctx context.Context, f AuditFilters, limit, offset int) ([]AuditItem, int, error) {
	action, search := f.params()
	rows, err := s.q.ListAudit(ctx, db.ListAuditParams{
		Action: action, ActorID: f.ActorID, Since: f.Since, Until: f.Until, Search: search,
		MaxRows: int32(limit), Skip: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("lecture du journal : %w", err)
	}
	total, err := s.q.CountAudit(ctx, db.CountAuditParams{
		Action: action, ActorID: f.ActorID, Since: f.Since, Until: f.Until, Search: search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("comptage du journal : %w", err)
	}
	out := make([]AuditItem, 0, len(rows))
	for _, r := range rows {
		item := AuditItem{
			ID: r.ID, At: r.At, ActorID: r.ActorID, ActorEmail: r.ActorEmail, Action: r.Action,
			TargetType: r.TargetType, TargetID: r.TargetID, IP: r.Ip, UserAgent: r.UserAgent, Details: map[string]any{},
		}
		_ = json.Unmarshal(r.Details, &item.Details)
		out = append(out, item)
	}
	return out, int(total), nil
}

// Purge efface les lignes plus vieilles que la retention. La table ne
// l'admet que declare dans la transaction.
func (s *AuditService) Purge(ctx context.Context) (int64, error) {
	if s.retention <= 0 {
		return 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.AllowAuditPurge(ctx); err != nil {
		return 0, err
	}
	n, err := q.PurgeAudit(ctx, int32(s.retention.Hours()/24))
	if err != nil {
		return 0, err
	}
	return n, tx.Commit(ctx)
}
