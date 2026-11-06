package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Liaison d'un compte Discord par code a usage unique.
//
// Le compte connecte tire un code ; le membre le tape dans Discord (/lier), et
// le bot le transmet avec l'identifiant Discord du membre. Taper le code prouve
// qu'on tient les deux comptes. L'API n'appelle jamais Discord : ni OAuth, ni
// appel externe pendant une requete.
//
// Pour le bot, une liaison n'existe que si son compte est actif : desactive,
// le compte perd ses roles sur Discord ; reactive, il les retrouve.

// DiscordCodeTTL : duree de vie d'un code.
const DiscordCodeTTL = 10 * time.Minute

var (
	// ErrLinkCodeInvalid ne distingue pas un code inconnu d'un code expire,
	// deja utilise ou remplace : l'ecart renseignerait qui essaie des codes.
	ErrLinkCodeInvalid = &domain.Error{
		Status: http.StatusNotFound, Code: "LINK_CODE_INVALID",
		Message: "Code inconnu ou expiré : générez-en un nouveau depuis Piilot",
	}
	ErrDiscordAlreadyLinked = &domain.Error{
		Status: http.StatusConflict, Code: "DISCORD_ALREADY_LINKED",
		Message: "Ce compte Discord est déjà lié à un autre compte Piilot",
	}
	ErrDiscordLinkNotFound = &domain.Error{
		Status: http.StatusNotFound, Code: "DISCORD_LINK_NOT_FOUND",
		Message: "Aucun compte lié à cet identifiant Discord",
	}
)

var discordUserIDPattern = regexp.MustCompile(`^[0-9]{17,20}$`)

// ValidDiscordUserID dit si une chaine a la forme d'un identifiant Discord.
func ValidDiscordUserID(id string) bool { return discordUserIDPattern.MatchString(id) }

// DiscordStatus est la carte « Discord » du compte connecte.
type DiscordStatus struct {
	// Au moins une cle du bot active : sans elle, un code ne servirait a rien
	// et l'ecran peut masquer la carte.
	Available     bool       `json:"available"`
	Linked        bool       `json:"linked"`
	DiscordUserID *string    `json:"discord_user_id"`
	LinkedAt      *time.Time `json:"linked_at"`
}

// DiscordCode est un code tire, sous sa forme affichee (XXXX-XXXX).
type DiscordCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DiscordLink est une liaison telle que le bot la voit. Le code du role
// seulement : ni nom, ni adresse. Le client du compte pourra s'y ajouter.
type DiscordLink struct {
	DiscordUserID string    `json:"discord_user_id"`
	Role          string    `json:"role"`
	LinkedAt      time.Time `json:"linked_at"`
}

// DiscordLinkCreated est la reponse au rachat d'un code. Replaced porte
// l'ancien identifiant Discord du compte, pour que le bot lui retire ses roles.
type DiscordLinkCreated struct {
	DiscordLink
	ReplacedDiscordUserID *string `json:"replaced_discord_user_id"`
	// Le compte lie : pour le journal d'audit, jamais pour le bot.
	UserID uuid.UUID `json:"-"`
}

// DiscordLinkPage est une page de la liste complete, pour une reconciliation.
// EventsCursor est le curseur du flux au moment de la lecture : garde depuis la
// premiere page, il dit d'ou reprendre le flux une fois la liste parcourue.
type DiscordLinkPage struct {
	Items        []DiscordLink `json:"items"`
	NextAfter    *string       `json:"next_after"`
	EventsCursor string        `json:"events_cursor"`
}

// DiscordLinkExport est la part de la liaison dans l'export d'un compte.
type DiscordLinkExport struct {
	DiscordUserID string    `json:"discord_user_id"`
	LinkedAt      time.Time `json:"linked_at"`
}

// DiscordService porte la liaison, cote compte et cote bot.
type DiscordService struct {
	pool    *pgxpool.Pool
	q       *db.Queries
	codeKey []byte
	rateKey []byte
}

// NewDiscordService construit le service. Les codes, et les membres dans le
// compteur d'essais, sont haches sous des cles derivees du secret applicatif.
func NewDiscordService(pool *pgxpool.Pool, secret []byte) *DiscordService {
	return &DiscordService{
		pool: pool, q: db.New(pool),
		codeKey: security.DeriveKey(secret, "discord-link-code"),
		rateKey: security.DeriveKey(secret, "discord-redeem-rate"),
	}
}

// RateKey designe un membre dans le compteur d'essais sans y ecrire son
// identifiant. Sous cle : Redis seul ne permet pas de verifier qu'un
// identifiant connu a essaye des codes.
func (s *DiscordService) RateKey(discordUserID string) string {
	mac := hmac.New(sha256.New, s.rateKey)
	mac.Write([]byte(discordUserID))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// Status rend la carte du compte connecte.
func (s *DiscordService) Status(ctx context.Context, userID uuid.UUID) (DiscordStatus, error) {
	available, err := s.q.HasActiveBotKey(ctx)
	if err != nil {
		return DiscordStatus{}, fmt.Errorf("lecture des cles du bot : %w", err)
	}
	out := DiscordStatus{Available: available}

	link, err := s.q.GetDiscordLinkByUser(ctx, userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return DiscordStatus{}, fmt.Errorf("lecture de la liaison : %w", err)
	default:
		out.Linked = true
		out.DiscordUserID = &link.DiscordUserID
		out.LinkedAt = &link.LinkedAt
	}
	return out, nil
}

// CreateCode tire un code pour le compte, et invalide le precedent.
func (s *DiscordService) CreateCode(ctx context.Context, userID uuid.UUID) (DiscordCode, error) {
	// Deux codes actifs identiques sont improbables a 40 bits, pas impossibles :
	// l'index unique refuse le second, on tire a nouveau.
	for range 3 {
		code, err := security.NewLinkCode()
		if err != nil {
			return DiscordCode{}, err
		}
		expires := time.Now().Add(DiscordCodeTTL).UTC().Truncate(time.Second)
		err = s.q.UpsertDiscordLinkCode(ctx, db.UpsertDiscordLinkCodeParams{
			UserID: userID, CodeHash: security.HashLinkCode(s.codeKey, code), ExpiresAt: expires,
		})
		if isUniqueViolation(err) {
			continue
		}
		if err != nil {
			return DiscordCode{}, fmt.Errorf("enregistrement du code : %w", err)
		}
		return DiscordCode{Code: security.FormatLinkCode(code), ExpiresAt: expires}, nil
	}
	return DiscordCode{}, errors.New("tirage du code : trois collisions de suite")
}

// Unlink supprime la liaison du compte. Sans liaison, ne fait rien.
func (s *DiscordService) Unlink(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.forget(ctx, s.q.WithTx(tx), userID, false); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Forget efface la liaison et le code d'un compte qu'on supprime, dans la
// transaction de la suppression, et dit au bot de retirer ses roles. A
// brancher sur la suppression des comptes (RGPD, 1.5).
func (s *DiscordService) Forget(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	return s.forget(ctx, s.q.WithTx(tx), userID, true)
}

func (s *DiscordService) forget(ctx context.Context, q *db.Queries, userID uuid.UUID, withCode bool) error {
	// Ordre des verrous de queries/discord.sql : le code, le compte, la
	// liaison, puis le compteur.
	if withCode {
		if err := q.DeleteDiscordLinkCodes(ctx, userID); err != nil {
			return fmt.Errorf("effacement du code : %w", err)
		}
	}
	if err := q.LockUser(ctx, userID); err != nil {
		return fmt.Errorf("verrou du compte : %w", err)
	}
	discordUserID, err := q.DeleteDiscordLinkByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("effacement de la liaison : %w", err)
	}
	return emitDiscordRemoved(ctx, q, discordUserID)
}

// Export rend la liaison d'un compte pour son export, ou nil.
func (s *DiscordService) Export(ctx context.Context, userID uuid.UUID) (*DiscordLinkExport, error) {
	link, err := s.q.GetDiscordLinkByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lecture de la liaison : %w", err)
	}
	return &DiscordLinkExport{DiscordUserID: link.DiscordUserID, LinkedAt: link.LinkedAt}, nil
}

// ValidateRedeem verifie la forme d'un rachat, avant toute limitation de debit
// ou lecture en base.
func ValidateRedeem(code, discordUserID string) error {
	details := map[string]any{}
	if _, ok := security.NormalizeLinkCode(code); !ok {
		details["code"] = "Un code de 8 caractères, au format XXXX-XXXX"
	}
	if !ValidDiscordUserID(discordUserID) {
		details["discord_user_id"] = "Un identifiant Discord : 17 à 20 chiffres, en chaîne"
	}
	if len(details) > 0 {
		return domain.ErrValidation.WithDetails(details)
	}
	return nil
}

// Redeem rachete un code au nom d'un membre Discord et lie les deux comptes.
//
// Le code est consomme des qu'il est reconnu, meme si la liaison echoue
// ensuite : un code ne sert qu'une fois, et un 409 ne doit pas permettre de
// confirmer qu'un code est bon pour le retenter depuis un autre compte.
func (s *DiscordService) Redeem(ctx context.Context, code, discordUserID string) (DiscordLinkCreated, error) {
	if err := ValidateRedeem(code, discordUserID); err != nil {
		return DiscordLinkCreated{}, err
	}
	normalized, _ := security.NormalizeLinkCode(code)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DiscordLinkCreated{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	// keep valide ce qui a ete fait — la consommation du code — et rend le
	// refus.
	keep := func(refusal error) (DiscordLinkCreated, error) {
		if err := tx.Commit(ctx); err != nil {
			return DiscordLinkCreated{}, fmt.Errorf("validation : %w", err)
		}
		return DiscordLinkCreated{}, refusal
	}

	consumed, err := q.ConsumeDiscordLinkCode(ctx, security.HashLinkCode(s.codeKey, normalized))
	if errors.Is(err, pgx.ErrNoRows) {
		return DiscordLinkCreated{}, ErrLinkCodeInvalid
	}
	if err != nil {
		return DiscordLinkCreated{}, fmt.Errorf("lecture du code : %w", err)
	}
	if !time.Now().Before(consumed.ExpiresAt) {
		return keep(ErrLinkCodeInvalid)
	}
	userID := consumed.UserID

	// Le membre Discord d'abord : deux rachats du meme membre passent l'un
	// apres l'autre, et le second voit la liaison du premier. Avant le compte,
	// pour que l'ordre des verrous reste le meme quel que soit le compte
	// auquel le membre est deja lie.
	if err := q.LockDiscordMember(ctx, discordUserID); err != nil {
		return DiscordLinkCreated{}, fmt.Errorf("verrou du membre : %w", err)
	}

	// Le compte, verrouille : un changement de role ou une desactivation
	// concurrents attendent la fin de la liaison.
	role, err := q.LockActiveUserRole(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return keep(ErrLinkCodeInvalid)
	}
	if err != nil {
		return DiscordLinkCreated{}, fmt.Errorf("lecture du compte : %w", err)
	}

	owner, err := q.GetDiscordLinkOwner(ctx, discordUserID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return DiscordLinkCreated{}, fmt.Errorf("lecture de la liaison existante : %w", err)
	case owner.UserID == userID:
		// Deja lie a ce compte : rien ne change, rien a dire au bot.
		link, err := q.GetDiscordLinkByUser(ctx, userID)
		if err != nil {
			return DiscordLinkCreated{}, fmt.Errorf("lecture de la liaison : %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return DiscordLinkCreated{}, fmt.Errorf("validation : %w", err)
		}
		return DiscordLinkCreated{
			DiscordLink: DiscordLink{DiscordUserID: discordUserID, Role: role, LinkedAt: link.LinkedAt}, UserID: userID,
		}, nil
	case owner.Active:
		return keep(ErrDiscordAlreadyLinked)
	default:
		// Lie a un compte desactive ou supprime : pour le bot, cette liaison
		// n'existe deja plus. Elle ne doit pas bloquer la nouvelle.
		if err := q.DeleteDiscordLinkByDiscordID(ctx, discordUserID); err != nil {
			return DiscordLinkCreated{}, fmt.Errorf("effacement de la liaison perimee : %w", err)
		}
	}

	out := DiscordLinkCreated{UserID: userID}
	previous, err := q.DeleteDiscordLinkByUser(ctx, userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return DiscordLinkCreated{}, fmt.Errorf("remplacement de la liaison : %w", err)
	default:
		out.ReplacedDiscordUserID = &previous
	}

	// Le verrou du membre rend toute liaison concurrente de ce membre visible
	// plus haut : l'index unique ne peut plus refuser l'insertion.
	linkedAt, err := q.InsertDiscordLink(ctx, db.InsertDiscordLinkParams{UserID: userID, DiscordUserID: discordUserID})
	if err != nil {
		return DiscordLinkCreated{}, fmt.Errorf("enregistrement de la liaison : %w", err)
	}

	// Les evenements en dernier : le compteur est le dernier verrou pris.
	if out.ReplacedDiscordUserID != nil {
		if err := emitDiscordRemoved(ctx, q, *out.ReplacedDiscordUserID); err != nil {
			return DiscordLinkCreated{}, err
		}
	}
	if err := emitDiscordUpserted(ctx, q, discordUserID, role); err != nil {
		return DiscordLinkCreated{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DiscordLinkCreated{}, fmt.Errorf("validation : %w", err)
	}

	out.DiscordLink = DiscordLink{DiscordUserID: discordUserID, Role: role, LinkedAt: linkedAt}
	return out, nil
}

// Link rend la liaison d'un membre Discord, si son compte est actif.
func (s *DiscordService) Link(ctx context.Context, discordUserID string) (DiscordLink, error) {
	if !ValidDiscordUserID(discordUserID) {
		return DiscordLink{}, domain.ErrValidation.WithDetails(map[string]any{
			"discord_user_id": "Un identifiant Discord : 17 à 20 chiffres",
		})
	}
	row, err := s.q.GetActiveDiscordLink(ctx, discordUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return DiscordLink{}, ErrDiscordLinkNotFound
	}
	if err != nil {
		return DiscordLink{}, fmt.Errorf("lecture de la liaison : %w", err)
	}
	return DiscordLink{DiscordUserID: row.DiscordUserID, Role: row.Role, LinkedAt: row.LinkedAt}, nil
}

// Links rend une page de toutes les liaisons actives, par identifiant Discord
// croissant, apres le curseur donne.
func (s *DiscordService) Links(ctx context.Context, after string, limit int) (DiscordLinkPage, error) {
	// Le curseur du flux et la page viennent de la meme photo de la base : ce
	// qui change ensuite arrive par le flux, a partir de ce curseur.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return DiscordLinkPage{}, fmt.Errorf("ouverture de la lecture : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	cursor, err := q.GetBotEventCursor(ctx)
	if err != nil {
		return DiscordLinkPage{}, fmt.Errorf("lecture du compteur : %w", err)
	}
	// Une ligne de plus que demande : elle dit s'il reste une page, sans
	// compter toute la table.
	rows, err := q.ListActiveDiscordLinks(ctx, db.ListActiveDiscordLinksParams{After: after, PageSize: int32(limit + 1)})
	if err != nil {
		return DiscordLinkPage{}, fmt.Errorf("lecture des liaisons : %w", err)
	}

	page := DiscordLinkPage{Items: make([]DiscordLink, 0, min(len(rows), limit)), EventsCursor: strconv.FormatInt(cursor.LastID, 10)}
	for i, r := range rows {
		if i == limit {
			last := page.Items[limit-1].DiscordUserID
			page.NextAfter = &last
			break
		}
		page.Items = append(page.Items, DiscordLink{DiscordUserID: r.DiscordUserID, Role: r.Role, LinkedAt: r.LinkedAt})
	}
	return page, nil
}

// Purge efface les codes expires.
func (s *DiscordService) Purge(ctx context.Context) error {
	if _, err := s.q.PurgeExpiredDiscordLinkCodes(ctx); err != nil {
		return fmt.Errorf("purge des codes de liaison : %w", err)
	}
	return nil
}
