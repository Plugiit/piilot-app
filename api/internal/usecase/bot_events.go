package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
)

// Le flux d'evenements que le bot relit.
//
// Generique : un type et des donnees minimales, ecrits dans la transaction du
// changement qu'ils racontent. Le bot le parcourt par curseur et applique ce
// qu'il lit ; chaque evenement est idempotent, le relire ne fait pas de mal.
// Il ignore les types qu'il ne connait pas : c'est ce qui laissera entrer de
// nouveaux evenements sans changer le contrat.

// Types d'evenements emis aujourd'hui.
const (
	BotEventDiscordLinkUpserted = "discord_link.upserted"
	BotEventDiscordLinkRemoved  = "discord_link.removed"
)

// BotEventRetention : au-dela, un evenement est purge. Un bot arrete plus
// longtemps reprend par la liste complete des liaisons.
const BotEventRetention = 30 * 24 * time.Hour

// BotPageMax borne les pages du flux et de la liste des liaisons.
const BotPageMax = 100

// ErrEventsCursorExpired : le curseur precede la purge, des evenements manquent.
var ErrEventsCursorExpired = &domain.Error{
	Status: http.StatusGone, Code: "EVENTS_CURSOR_EXPIRED",
	Message: "Ce curseur précède les événements conservés : reprenez la liste complète des liaisons",
}

// BotEvent est un evenement du flux. L'identifiant est un curseur opaque, en
// chaine : le bot le garde et le rend tel quel.
type BotEvent struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// BotEventPage est une page du flux. NextAfter n'est jamais vide : sur une
// page vide, c'est le curseur recu, a reposer tel quel au prochain appel.
type BotEventPage struct {
	Items     []BotEvent `json:"items"`
	NextAfter string     `json:"next_after"`
}

// BotEventService sert le flux et le purge.
type BotEventService struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewBotEventService construit le service.
func NewBotEventService(pool *pgxpool.Pool) *BotEventService {
	return &BotEventService{pool: pool, q: db.New(pool)}
}

// Events rend les evenements qui suivent un curseur. Sans curseur, le flux
// part du plus ancien evenement conserve.
func (s *BotEventService) Events(ctx context.Context, after *int64, limit int) (BotEventPage, error) {
	// Une seule photo de la base pour le filigrane et la page : une purge
	// passee entre les deux lectures emporterait des evenements sans que le
	// curseur paraisse perime.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return BotEventPage{}, fmt.Errorf("ouverture de la lecture : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	cursor, err := q.GetBotEventCursor(ctx)
	if err != nil {
		return BotEventPage{}, fmt.Errorf("lecture du compteur : %w", err)
	}
	from := cursor.PurgedThrough
	if after != nil {
		if *after < cursor.PurgedThrough {
			return BotEventPage{}, ErrEventsCursorExpired
		}
		from = *after
	}

	rows, err := q.ListBotEventsAfter(ctx, db.ListBotEventsAfterParams{After: from, PageSize: int32(limit)})
	if err != nil {
		return BotEventPage{}, fmt.Errorf("lecture du flux : %w", err)
	}

	page := BotEventPage{Items: make([]BotEvent, 0, len(rows)), NextAfter: strconv.FormatInt(from, 10)}
	for _, r := range rows {
		page.Items = append(page.Items, BotEvent{
			ID: strconv.FormatInt(r.ID, 10), Type: r.Type, OccurredAt: r.CreatedAt, Data: r.Data,
		})
		page.NextAfter = strconv.FormatInt(r.ID, 10)
	}
	return page, nil
}

// Purge efface les evenements trop vieux et avance le filigrane.
func (s *BotEventService) Purge(ctx context.Context) error {
	if _, err := s.q.PurgeBotEvents(ctx, time.Now().Add(-BotEventRetention)); err != nil {
		return fmt.Errorf("purge du flux du bot : %w", err)
	}
	return nil
}

// emitBotEvent ecrit un evenement dans la transaction de q. Jamais hors d'une
// transaction qui porte aussi le changement : un evenement sans changement, ou
// l'inverse, desynchroniserait le bot.
func emitBotEvent(ctx context.Context, q *db.Queries, typ string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("evenement %s : %w", typ, err)
	}
	if err := q.EmitBotEvent(ctx, db.EmitBotEventParams{Type: typ, Data: payload}); err != nil {
		return fmt.Errorf("emission de %s : %w", typ, err)
	}
	return nil
}

func emitDiscordUpserted(ctx context.Context, q *db.Queries, discordUserID, role string) error {
	return emitBotEvent(ctx, q, BotEventDiscordLinkUpserted, struct {
		DiscordUserID string `json:"discord_user_id"`
		Role          string `json:"role"`
	}{discordUserID, role})
}

func emitDiscordRemoved(ctx context.Context, q *db.Queries, discordUserID string) error {
	return emitBotEvent(ctx, q, BotEventDiscordLinkRemoved, struct {
		DiscordUserID string `json:"discord_user_id"`
	}{discordUserID})
}

// syncDiscordLink dit au bot ce que change un nouveau role ou un nouvel etat
// du compte. A appeler dans la transaction du changement, apres l'ecriture sur
// users : la ligne du compte est alors verrouillee, et l'ordre des verrous
// (compte, liaison, compteur) reste celui de la liaison elle-meme.
//
// Un compte desactive perd ses roles sur Discord ; reactive, il les retrouve.
// La liaison reste en base dans les deux cas.
func syncDiscordLink(ctx context.Context, q *db.Queries, userID uuid.UUID) error {
	link, err := q.GetDiscordLinkByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lecture de la liaison Discord : %w", err)
	}

	role, err := q.LockActiveUserRole(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return emitDiscordRemoved(ctx, q, link.DiscordUserID)
	}
	if err != nil {
		return fmt.Errorf("lecture du compte : %w", err)
	}
	return emitDiscordUpserted(ctx, q, link.DiscordUserID, role)
}
