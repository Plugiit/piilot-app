package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Cles du bot : son acces machine a l'API.
//
// Pas de faux compte avec un mot de passe : une cle porte le role systeme
// « bot », dont les permissions se verifient comme celles d'un compte. Elle ne
// vaut que sous /api/v1/bot, et se cree ou se revoque depuis le terminal du
// conteneur (commande bot-key) : pas d'ecran pour l'instant.

// ErrBotKeyNameTaken : une cle active porte deja ce nom.
var ErrBotKeyNameTaken = &domain.Error{
	Status: http.StatusConflict, Code: "BOT_KEY_NAME_TAKEN",
	Message: "Une clé active porte déjà ce nom",
}

var botKeyName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// BotKey decrit une cle, jamais sa valeur.
type BotKey struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// BotKeyService cree, liste, revoque et verifie les cles.
type BotKeyService struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewBotKeyService construit le service.
func NewBotKeyService(pool *pgxpool.Pool) *BotKeyService {
	return &BotKeyService{pool: pool, q: db.New(pool)}
}

// Create tire une cle et rend sa valeur, a montrer une seule fois : seule son
// empreinte est gardee.
func (s *BotKeyService) Create(ctx context.Context, name string) (string, BotKey, error) {
	if !botKeyName.MatchString(name) {
		return "", BotKey{}, domain.ErrValidation.WithDetails(map[string]any{
			"name": "Minuscules, chiffres, point, tiret ou souligné, 64 caractères au plus",
		})
	}
	raw, hash, err := security.NewBotKey()
	if err != nil {
		return "", BotKey{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", BotKey{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.CreateBotKey(ctx, db.CreateBotKeyParams{Name: name, KeyHash: hash})
	if err != nil {
		if isUniqueViolation(err) {
			return "", BotKey{}, ErrBotKeyNameTaken
		}
		return "", BotKey{}, fmt.Errorf("creation de la cle : %w", err)
	}
	if err := auditBotKey(ctx, q, "bot_key.created", row.ID, row.Name); err != nil {
		return "", BotKey{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", BotKey{}, fmt.Errorf("validation : %w", err)
	}

	return raw, BotKey{ID: row.ID, Name: row.Name, Role: row.Role, CreatedAt: row.CreatedAt}, nil
}

// List rend les cles, actives d'abord.
func (s *BotKeyService) List(ctx context.Context) ([]BotKey, error) {
	rows, err := s.q.ListBotKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("lecture des cles : %w", err)
	}
	out := make([]BotKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, BotKey{
			ID: r.ID, Name: r.Name, Role: r.Role, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt,
		})
	}
	return out, nil
}

// Revoke retire une cle. Elle est refusee des la requete suivante.
func (s *BotKeyService) Revoke(ctx context.Context, id uuid.UUID) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.RevokeBotKey(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound.WithMessage("Aucune clé active avec cet identifiant")
	}
	if err != nil {
		return "", fmt.Errorf("revocation de la cle : %w", err)
	}
	if err := auditBotKey(ctx, q, "bot_key.revoked", row.ID, row.Name); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("validation : %w", err)
	}
	return row.Name, nil
}

// AuthenticateBotKey dit si une cle presentee est active, et sous quel role.
// Consomme par la garde HTTP, qui ne connait ni sqlc ni le schema.
func (s *BotKeyService) AuthenticateBotKey(ctx context.Context, raw string) (string, uuid.UUID, bool, error) {
	hash, ok := security.BotKeyHash(raw)
	if !ok {
		return "", uuid.Nil, false, nil
	}
	row, err := s.q.GetActiveBotKeyByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", uuid.Nil, false, nil
	}
	if err != nil {
		return "", uuid.Nil, false, fmt.Errorf("lecture de la cle : %w", err)
	}
	if err := s.q.TouchBotKey(ctx, row.ID); err != nil {
		return "", uuid.Nil, false, fmt.Errorf("date d'usage de la cle : %w", err)
	}
	return row.Role, row.ID, true, nil
}

// auditBotKey ecrit la creation ou la revocation au journal, dans la meme
// transaction. Sans acteur : le geste vient du terminal du conteneur.
func auditBotKey(ctx context.Context, q *db.Queries, action string, id uuid.UUID, name string) error {
	details, err := json.Marshal(map[string]any{"name": name})
	if err != nil {
		return err
	}
	if err := q.InsertAudit(ctx, db.InsertAuditParams{
		Action: action, TargetType: "bot_key", TargetID: id.String(), UserAgent: "bot-key", Details: details,
	}); err != nil {
		return fmt.Errorf("journal d'audit : %w", err)
	}
	return nil
}
