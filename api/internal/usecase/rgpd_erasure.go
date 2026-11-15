package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	mailer "github.com/plugiit/piilot-app/api/internal/mail"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/storage"
)

// Etats d'un effacement.
const (
	RgpdErasureScheduled = "scheduled"
	RgpdErasureCancelled = "cancelled"
	RgpdErasureRunning   = "running"
	RgpdErasureDone      = "done"
	RgpdErasureFailed    = "failed"
)

// Origines d'un effacement.
const (
	RgpdReasonRequest    = "request"
	RgpdReasonInactivity = "inactivity"
)

// Decisions de produit retenues pour la 1.5, a un seul endroit chacune pour
// pouvoir en changer sans toucher au reste.
//
//   - Effacer un CLIENT supprime tout : eraseClient.
//   - Effacer un COMPTE l'anonymise, le temps saisi reste : eraseUser.
//   - Pendant le delai de retractation, les comptes concernes sont desactives
//     tout de suite (rgpdDisableOnRequest). Pour une purge d'inactifs on ne
//     desactive pas : la connexion est justement ce qui annule l'effacement.
const rgpdDisableOnRequest = true

const (
	// rgpdInactiveBatch : comptes planifies par lecture, pour la purge.
	rgpdInactiveBatch = 100
	// rgpdInactiveMaxBatches borne une passe de la purge.
	rgpdInactiveMaxBatches = 50
)

// RgpdForgetter est ce que l'effacement attend de la liaison Discord.
type RgpdForgetter interface {
	Forget(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
}

// RgpdErasureItem est un effacement, tel que l'ecran le montre.
type RgpdErasureItem struct {
	ID           uuid.UUID      `json:"id"`
	SubjectType  string         `json:"subject_type"`
	SubjectID    uuid.UUID      `json:"subject_id"`
	SubjectLabel string         `json:"subject_label"`
	Reason       string         `json:"reason"`
	Status       string         `json:"status"`
	ScheduledFor time.Time      `json:"scheduled_for"`
	RequestedBy  *uuid.UUID     `json:"requested_by"`
	CancelledBy  *uuid.UUID     `json:"cancelled_by"`
	CancelledAt  *time.Time     `json:"cancelled_at"`
	ExecutedAt   *time.Time     `json:"executed_at"`
	Summary      map[string]any `json:"summary"`
	Error        string         `json:"error"`
	CreatedAt    time.Time      `json:"created_at"`
}

// RgpdErasurePage est une page de l'historique des effacements.
type RgpdErasurePage struct {
	Items    []RgpdErasureItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

// RgpdDeletedClient est un client supprime dans le CRM dont les donnees sont
// toujours conservees.
type RgpdDeletedClient struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RgpdDeletedClientPage est une page de clients supprimes.
type RgpdDeletedClientPage struct {
	Items    []RgpdDeletedClient `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// RgpdSettings expose les delais en vigueur.
type RgpdSettings struct {
	ErasureGraceHours    int  `json:"erasure_grace_hours"`
	InactiveAfterHours   int  `json:"inactive_after_hours"`
	InactivePurgeEnabled bool `json:"inactive_purge_enabled"`
}

// RgpdErasureService demande, annule et execute les effacements RGPD.
type RgpdErasureService struct {
	pool          *pgxpool.Pool
	q             *db.Queries
	files         storage.Store
	discord       RgpdForgetter
	grace         time.Duration
	inactiveAfter time.Duration
	log           *slog.Logger

	// Preavis des comptes inactifs : envoye seulement si l'envoi d'e-mails est
	// configure.
	mailEnabled bool
	loginURL    string

	now func() time.Time
	// afterStep, nil en production, laisse un test faire echouer une etape
	// precise pour verifier le retour arriere complet.
	afterStep func(step string) error
}

// NewRgpdErasureService construit le service. grace : delai de retractation ;
// inactiveAfter : inactivite au bout de laquelle un compte est planifie (0 :
// purge desactivee).
func NewRgpdErasureService(
	pool *pgxpool.Pool, files storage.Store, discord RgpdForgetter, grace, inactiveAfter time.Duration, log *slog.Logger,
) *RgpdErasureService {
	return &RgpdErasureService{
		pool: pool, q: db.New(pool), files: files, discord: discord,
		grace: grace, inactiveAfter: inactiveAfter, log: log, now: time.Now,
	}
}

// SetNotice active le preavis par e-mail des comptes inactifs. loginURL : la
// page ou se connecter pour annuler.
func (s *RgpdErasureService) SetNotice(loginURL string, mailEnabled bool) {
	s.loginURL, s.mailEnabled = loginURL, mailEnabled
}

// Settings rend les delais en vigueur.
func (s *RgpdErasureService) Settings() RgpdSettings {
	return RgpdSettings{
		ErasureGraceHours:    int(s.grace / time.Hour),
		InactiveAfterHours:   int(s.inactiveAfter / time.Hour),
		InactivePurgeEnabled: s.inactiveAfter > 0,
	}
}

func rgpdErasureItemOf(e db.RgpdErasure) RgpdErasureItem {
	summary := map[string]any{}
	if len(e.Summary) > 0 {
		_ = json.Unmarshal(e.Summary, &summary)
	}

	return RgpdErasureItem{
		ID: e.ID, SubjectType: e.SubjectType, SubjectID: e.SubjectID, SubjectLabel: e.SubjectLabel,
		Reason: e.Reason, Status: e.Status, ScheduledFor: e.ScheduledFor, RequestedBy: e.RequestedBy,
		CancelledBy: e.CancelledBy, CancelledAt: e.CancelledAt, ExecutedAt: e.ExecutedAt,
		Summary: summary, Error: e.Error, CreatedAt: e.CreatedAt,
	}
}

// Request planifie l'effacement d'un compte ou d'un client a la demande d'un
// admin. Les comptes concernes sont desactives tout de suite, leurs sessions
// fermees ; l'effacement lui-meme n'a lieu qu'a l'echeance.
func (s *RgpdErasureService) Request(ctx context.Context, actor *uuid.UUID, subjectType string, subjectID uuid.UUID) (RgpdErasureItem, error) {
	return s.request(ctx, actor, subjectType, subjectID, RgpdReasonRequest)
}

func (s *RgpdErasureService) request(
	ctx context.Context, actor *uuid.UUID, subjectType string, subjectID uuid.UUID, reason string,
) (RgpdErasureItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RgpdErasureItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	var (
		label   string
		userIDs []uuid.UUID
		prev    = map[string]*time.Time{}
	)

	switch subjectType {
	case RgpdSubjectUser:
		u, err := q.LockUserForErasure(ctx, subjectID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.DeletedAt != nil) {
			return RgpdErasureItem{}, domain.ErrNotFound.WithDetails(map[string]any{"subject_id": "Sujet introuvable"})
		}

		if err != nil {
			return RgpdErasureItem{}, fmt.Errorf("lecture du compte : %w", err)
		}

		// Garde-fou : il reste toujours un administrateur actif.
		if u.Role == "admin" && u.DisabledAt == nil {
			n, err := q.CountActiveAdmins(ctx)
			if err != nil {
				return RgpdErasureItem{}, fmt.Errorf("compte des administrateurs : %w", err)
			}

			if n <= 1 {
				return RgpdErasureItem{}, ErrLastAdmin
			}
		}

		names, err := q.RgpdUserLabel(ctx, subjectID)
		if err != nil {
			return RgpdErasureItem{}, fmt.Errorf("lecture du libelle : %w", err)
		}

		if label = names.Name; label == "" {
			label = names.Email
		}

		userIDs = append(userIDs, subjectID)
		prev[subjectID.String()] = u.DisabledAt
	case RgpdSubjectClient:
		c, err := q.LockClientForErasure(ctx, subjectID)
		if errors.Is(err, pgx.ErrNoRows) {
			return RgpdErasureItem{}, domain.ErrNotFound.WithDetails(map[string]any{"subject_id": "Sujet introuvable"})
		}

		if err != nil {
			return RgpdErasureItem{}, fmt.Errorf("lecture du client : %w", err)
		}

		label = c.Name

		users, err := q.ListClientUsersForErasure(ctx, &subjectID)
		if err != nil {
			return RgpdErasureItem{}, fmt.Errorf("lecture des comptes du portail : %w", err)
		}

		for _, u := range users {
			if u.DeletedAt == nil {
				userIDs = append(userIDs, u.ID)
				prev[u.ID.String()] = u.DisabledAt
			}
		}
	default:
		return RgpdErasureItem{}, domain.ErrValidation.WithDetails(map[string]any{"subject_type": "Type inconnu : client ou user"})
	}

	disable := rgpdDisableOnRequest && reason == RgpdReasonRequest

	var prevJSON []byte
	if disable {
		if prevJSON, err = json.Marshal(prev); err != nil {
			return RgpdErasureItem{}, fmt.Errorf("etat precedent : %w", err)
		}
	}

	e, err := q.CreateRgpdErasure(ctx, db.CreateRgpdErasureParams{
		SubjectType: subjectType, SubjectID: subjectID, SubjectLabel: truncateRunes(label, 200), Reason: reason,
		ScheduledFor: s.now().Add(s.grace), RequestedBy: actor, PrevDisabled: prevJSON,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return RgpdErasureItem{}, domain.ErrConflict.WithDetails(map[string]any{"subject_id": "Un effacement est déjà en cours pour ce sujet"})
		}

		return RgpdErasureItem{}, fmt.Errorf("creation de l'effacement : %w", err)
	}

	if disable {
		for _, id := range userIDs {
			if err := s.suspend(ctx, q, id); err != nil {
				return RgpdErasureItem{}, err
			}
		}
	}

	if reason == RgpdReasonInactivity && s.mailEnabled && subjectType == RgpdSubjectUser {
		if err := s.notify(ctx, q, subjectID, e.ScheduledFor); err != nil {
			return RgpdErasureItem{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return RgpdErasureItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return rgpdErasureItemOf(e), nil
}

// suspend desactive un compte et coupe tout ce qui agit en son nom, comme
// AccountService.Disable.
func (s *RgpdErasureService) suspend(ctx context.Context, q *db.Queries, id uuid.UUID) error {
	if err := q.DisableUserKeepingDate(ctx, id); err != nil {
		return fmt.Errorf("desactivation : %w", err)
	}

	if err := q.RevokeAllUserRefreshTokens(ctx, id); err != nil {
		return fmt.Errorf("revocation des sessions : %w", err)
	}

	if err := q.RevokeAllUserOAuthGrants(ctx, id); err != nil {
		return fmt.Errorf("revocation des connexions de Claude : %w", err)
	}

	if err := q.InvalidateUserPasswordResets(ctx, id); err != nil {
		return fmt.Errorf("annulation des liens de reinitialisation : %w", err)
	}

	return syncDiscordLink(ctx, q, id)
}

// notify met le preavis dans la file d'envoi.
func (s *RgpdErasureService) notify(ctx context.Context, q *db.Queries, userID uuid.UUID, deadline time.Time) error {
	u, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("lecture du compte : %w", err)
	}

	msg, err := mailer.ErasureNotice(u.Email, u.Firstname, s.loginURL, deadline)
	if err != nil {
		return fmt.Errorf("composition du preavis : %w", err)
	}

	return enqueue(ctx, q, msg)
}

// Cancel annule un effacement encore planifie et rend aux comptes leur etat
// d'avant la demande.
func (s *RgpdErasureService) Cancel(ctx context.Context, actor *uuid.UUID, id uuid.UUID) (RgpdErasureItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RgpdErasureItem{}, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	e, err := q.GetRgpdErasureForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return RgpdErasureItem{}, domain.ErrNotFound
	}

	if err != nil {
		return RgpdErasureItem{}, fmt.Errorf("lecture de l'effacement : %w", err)
	}

	if e.Status != RgpdErasureScheduled {
		return RgpdErasureItem{}, domain.ErrConflict.WithDetails(map[string]any{"status": "Seul un effacement planifié peut être annulé"})
	}

	var prev map[string]*time.Time
	if len(e.PrevDisabled) > 0 {
		if err := json.Unmarshal(e.PrevDisabled, &prev); err != nil {
			return RgpdErasureItem{}, fmt.Errorf("etat precedent illisible : %w", err)
		}
	}

	for rawID, disabledAt := range prev {
		userID, err := uuid.Parse(rawID)
		if err != nil {
			return RgpdErasureItem{}, fmt.Errorf("identifiant precedent illisible : %w", err)
		}

		// Un autre effacement planifie vise encore ce compte (lui-meme ou son
		// client) : le reactiver le ferait vivre pendant un delai qui court
		// toujours. Il reste desactive, et l'etat d'avant est confie a
		// l'effacement restant pour que son annulation le rende.
		others, err := q.ListOtherScheduledErasuresOfUser(ctx, db.ListOtherScheduledErasuresOfUserParams{Excluded: id, UserID: userID})
		if err != nil {
			return RgpdErasureItem{}, fmt.Errorf("lecture des autres effacements du compte : %w", err)
		}

		if len(others) > 0 {
			if err := handOverPrevDisabled(ctx, q, e, others, rawID, disabledAt); err != nil {
				return RgpdErasureItem{}, err
			}

			continue
		}

		if err := q.SetUserDisabledAt(ctx, db.SetUserDisabledAtParams{ID: userID, DisabledAt: disabledAt}); err != nil {
			return RgpdErasureItem{}, fmt.Errorf("restauration du compte : %w", err)
		}
		// Un compte lie retrouve (ou garde) ses roles sur Discord.
		if err := syncDiscordLink(ctx, q, userID); err != nil {
			return RgpdErasureItem{}, err
		}
	}

	if err := q.CancelRgpdErasure(ctx, db.CancelRgpdErasureParams{ID: id, CancelledBy: actor}); err != nil {
		return RgpdErasureItem{}, fmt.Errorf("annulation : %w", err)
	}

	e, err = q.GetRgpdErasureForUpdate(ctx, id)
	if err != nil {
		return RgpdErasureItem{}, fmt.Errorf("relecture de l'effacement : %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RgpdErasureItem{}, fmt.Errorf("validation de la transaction : %w", err)
	}

	return rgpdErasureItemOf(e), nil
}

// handOverPrevDisabled confie l'etat d'avant un compte aux effacements qui le
// visent encore. Une demande posterieure a celle qu'on annule a releve la
// desactivation de celle-ci comme « etat d'avant » : sans correction, son
// annulation laisserait le compte desactive pour toujours.
func handOverPrevDisabled(
	ctx context.Context, q *db.Queries, cancelled db.RgpdErasure, others []db.RgpdErasure, rawUserID string, disabledAt *time.Time,
) error {
	for _, o := range others {
		if !o.CreatedAt.After(cancelled.CreatedAt) {
			continue // Posterieure a l'autre : son etat d'avant est deja le bon.
		}

		prev := map[string]*time.Time{}
		if len(o.PrevDisabled) > 0 {
			if err := json.Unmarshal(o.PrevDisabled, &prev); err != nil {
				return fmt.Errorf("etat precedent illisible : %w", err)
			}
		}

		prev[rawUserID] = disabledAt

		raw, err := json.Marshal(prev)
		if err != nil {
			return fmt.Errorf("etat precedent : %w", err)
		}

		if err := q.SetRgpdErasurePrevDisabled(ctx, db.SetRgpdErasurePrevDisabledParams{ID: o.ID, PrevDisabled: raw}); err != nil {
			return fmt.Errorf("transmission de l'etat precedent : %w", err)
		}
	}

	return nil
}

// RgpdErasureFilter restreint l'historique : chaque champ vide ou nil laisse
// le critere ouvert.
type RgpdErasureFilter struct {
	Status      string
	SubjectType string
	SubjectID   *uuid.UUID
}

// List rend une page de l'historique, les plus recents d'abord, restreint par
// le filtre (statut, type et identifiant du sujet, combinables).
func (s *RgpdErasureService) List(ctx context.Context, f RgpdErasureFilter, page int) (RgpdErasurePage, error) {
	if page < 1 {
		page = 1
	}

	var status, subjectType *string

	if f.Status != "" {
		switch f.Status {
		case RgpdErasureScheduled, RgpdErasureCancelled, RgpdErasureRunning, RgpdErasureDone, RgpdErasureFailed:
			status = &f.Status
		default:
			return RgpdErasurePage{}, domain.ErrValidation.WithDetails(map[string]any{"status": "Statut inconnu"})
		}
	}

	if f.SubjectType != "" {
		switch f.SubjectType {
		case RgpdSubjectClient, RgpdSubjectUser:
			subjectType = &f.SubjectType
		default:
			return RgpdErasurePage{}, domain.ErrValidation.WithDetails(map[string]any{"subject_type": "Type inconnu : client ou user"})
		}
	}

	filter := db.CountRgpdErasuresParams{Status: status, SubjectType: subjectType, SubjectID: f.SubjectID}

	rows, err := s.q.ListRgpdErasures(ctx, db.ListRgpdErasuresParams{
		Status: status, SubjectType: subjectType, SubjectID: f.SubjectID, PageSize: RgpdPageSize, PageOffset: int32((page - 1) * RgpdPageSize),
	})
	if err != nil {
		return RgpdErasurePage{}, fmt.Errorf("lecture des effacements : %w", err)
	}

	total, err := s.q.CountRgpdErasures(ctx, filter)
	if err != nil {
		return RgpdErasurePage{}, fmt.Errorf("compte des effacements : %w", err)
	}

	items := make([]RgpdErasureItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, rgpdErasureItemOf(r))
	}

	return RgpdErasurePage{Items: items, Total: total, Page: page, PageSize: RgpdPageSize}, nil
}

// DeletedClients liste les clients supprimes dans le CRM (suppression
// logique) : leurs donnees existent toujours, l'ecran RGPD doit pouvoir les
// exporter ou les effacer.
func (s *RgpdErasureService) DeletedClients(ctx context.Context, page int) (RgpdDeletedClientPage, error) {
	if page < 1 {
		page = 1
	}

	rows, err := s.q.ListDeletedClients(ctx, db.ListDeletedClientsParams{
		PageSize: RgpdPageSize, PageOffset: int32((page - 1) * RgpdPageSize),
	})
	if err != nil {
		return RgpdDeletedClientPage{}, fmt.Errorf("lecture des clients supprimes : %w", err)
	}

	total, err := s.q.CountDeletedClients(ctx)
	if err != nil {
		return RgpdDeletedClientPage{}, fmt.Errorf("compte des clients supprimes : %w", err)
	}

	items := make([]RgpdDeletedClient, 0, len(rows))
	for _, r := range rows {
		if r.DeletedAt != nil {
			items = append(items, RgpdDeletedClient{ID: r.ID, Name: r.Name, DeletedAt: *r.DeletedAt})
		}
	}

	return RgpdDeletedClientPage{Items: items, Total: total, Page: page, PageSize: RgpdPageSize}, nil
}

// erasure accumule ce qu'une execution a fait : des compteurs (jamais de nom)
// et les cles des fichiers a supprimer apres le COMMIT.
type erasure struct {
	counts map[string]int64
	keys   []string
}

func (r *erasure) add(name string, n int64) {
	if n > 0 {
		r.counts[name] += n
	}
}

func (r *erasure) merge(o *erasure) {
	for k, v := range o.counts {
		r.add(k, v)
	}

	r.keys = append(r.keys, o.keys...)
}

func (s *RgpdErasureService) step(name string) error {
	if s.afterStep == nil {
		return nil
	}

	return s.afterStep(name)
}

// Execute execute le prochain effacement arrive a echeance, en une seule
// transaction (vrai s'il y en avait un). La ligne est reservee sous
// `FOR UPDATE SKIP LOCKED` : deux instances n'executent jamais le meme.
// Les fichiers ne partent qu'apres le COMMIT ; un echec de suppression est
// logue, sans rien defaire.
func (s *RgpdErasureService) Execute(ctx context.Context) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("ouverture de la transaction : %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := s.q.WithTx(tx)

	e, err := q.ClaimDueRgpdErasure(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("reservation d'un effacement : %w", err)
	}

	res, err := s.run(ctx, tx, q, e)
	if err != nil {
		_ = tx.Rollback(ctx)
		s.fail(ctx, e.ID, err)

		return true, fmt.Errorf("effacement %s : %w", e.ID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("validation de l'effacement %s : %w", e.ID, err)
	}

	s.removeFiles(res.keys)

	return true, nil
}

// fail note l'echec hors de la transaction annulee : rien n'a ete efface, le
// detail est dans les logs, et l'effacement ne se rejoue pas tout seul.
func (s *RgpdErasureService) fail(ctx context.Context, id uuid.UUID, cause error) {
	s.log.Error("effacement rgpd en echec", "id", id, "error", cause)

	if ctx.Err() != nil {
		// Arret en cours : l'effacement reste planifie, il sera repris.
		return
	}

	if err := s.q.MarkRgpdErasureFailed(ctx, db.MarkRgpdErasureFailedParams{ID: id, Error: rgpdFailInternal}); err != nil {
		s.log.Error("statut d'echec de l'effacement non ecrit", "id", id, "error", err)
	}
}

func (s *RgpdErasureService) removeFiles(keys []string) {
	seen := map[string]bool{}

	for _, k := range keys {
		if k == "" || seen[k] {
			continue
		}

		seen[k] = true

		if err := s.files.Remove(k); err != nil {
			s.log.Warn("fichier non supprime apres un effacement rgpd", "key", k, "error", err)
		}
	}
}

// run fait tout le travail d'un effacement dans la transaction recue.
func (s *RgpdErasureService) run(ctx context.Context, tx pgx.Tx, q *db.Queries, e db.RgpdErasure) (*erasure, error) {
	var (
		res *erasure
		err error
	)

	switch e.SubjectType {
	case RgpdSubjectUser:
		res, err = s.eraseUser(ctx, tx, q, e.SubjectID, false)
	case RgpdSubjectClient:
		res, err = s.eraseClient(ctx, tx, q, e.SubjectID)
	default:
		err = fmt.Errorf("type de sujet inconnu %q", e.SubjectType)
	}

	if err != nil {
		return nil, err
	}

	// Les archives d'export du sujet contiennent ses donnees.
	keys, err := q.EraseExportKeysOfSubject(ctx, db.EraseExportKeysOfSubjectParams{SubjectType: e.SubjectType, SubjectID: e.SubjectID})
	if err != nil {
		return nil, fmt.Errorf("lecture des exports du sujet : %w", err)
	}

	res.keys = append(res.keys, keys...)
	res.add("exports", int64(len(keys)))

	if _, err := q.EraseRedactExportsOfSubject(ctx, db.EraseRedactExportsOfSubjectParams{SubjectType: e.SubjectType, SubjectID: e.SubjectID}); err != nil {
		return nil, fmt.Errorf("effacement du libelle des exports : %w", err)
	}

	if err := s.step("exports"); err != nil {
		return nil, err
	}

	files := int64(len(res.keys))

	summary, err := json.Marshal(map[string]any{"counts": res.counts, "files": files})
	if err != nil {
		return nil, fmt.Errorf("resume : %w", err)
	}

	// Ids et compteurs, aucun nom : le journal est en ajout seul.
	details, err := json.Marshal(map[string]any{
		"erasure_id": e.ID, "subject_type": e.SubjectType, "subject_id": e.SubjectID, "reason": e.Reason,
		"counts": res.counts, "files": files,
	})
	if err != nil {
		return nil, fmt.Errorf("details de l'audit : %w", err)
	}

	if err := q.InsertAudit(ctx, db.InsertAuditParams{
		Action: "rgpd.erasure_executed", TargetType: "rgpd_erasure", TargetID: e.ID.String(), Details: details,
	}); err != nil {
		return nil, fmt.Errorf("audit : %w", err)
	}

	if err := q.MarkRgpdErasureDone(ctx, db.MarkRgpdErasureDoneParams{ID: e.ID, Summary: summary}); err != nil {
		return nil, fmt.Errorf("fin de l'effacement : %w", err)
	}

	if err := s.step("done"); err != nil {
		return nil, err
	}

	return res, nil
}

// eraseUser ANONYMISE un compte : la ligne reste (time_entries.user_id est en
// cascade : la supprimer effacerait le temps saisi), tout ce qui identifie la
// personne part. detach : retire aussi le lien au client (effacement d'un
// client, dont la suppression est interdite tant qu'un compte le designe).
func (s *RgpdErasureService) eraseUser(ctx context.Context, tx pgx.Tx, q *db.Queries, id uuid.UUID, detach bool) (*erasure, error) {
	res := &erasure{counts: map[string]int64{}}

	// En premier : Forget prend ses verrous dans un ordre fixe (code, compte,
	// liaison), avant toute ecriture sur `users`.
	if err := s.discord.Forget(ctx, tx, id); err != nil {
		return nil, fmt.Errorf("liaison Discord : %w", err)
	}

	if err := s.step("discord"); err != nil {
		return nil, err
	}

	u, err := q.LockUserForErasure(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		res.add("already_erased", 1)
		return res, nil
	}

	if err != nil {
		return nil, fmt.Errorf("lecture du compte : %w", err)
	}

	anonymize := db.EraseAnonymizeUserParams{
		ID: id, Email: "supprime-" + id.String() + "@invalid", Detach: detach,
	}

	if u.DeletedAt != nil {
		// Deja anonymise (par une autre demande) : seul le lien au client reste a couper.
		res.add("already_erased", 1)

		if err := q.EraseAnonymizeUser(ctx, anonymize); err != nil {
			return nil, fmt.Errorf("anonymisation : %w", err)
		}

		return res, nil
	}

	email := strings.ToLower(u.Email)

	type deletion struct {
		name string
		run  func() (int64, error)
	}

	steps := []deletion{
		{"refresh_tokens", func() (int64, error) { return q.EraseDeleteRefreshTokens(ctx, id) }},
		{"oauth_grants", func() (int64, error) { return q.EraseDeleteOAuthGrants(ctx, id) }},
		{"notifications", func() (int64, error) { return q.EraseDeleteNotifications(ctx, id) }},
		{"timers", func() (int64, error) { return q.EraseDeleteTimers(ctx, id) }},
		{"favorites", func() (int64, error) { return q.EraseDeleteFavorites(ctx, id) }},
		{"password_resets", func() (int64, error) { return q.EraseDeletePasswordResets(ctx, id) }},
		{"invitations", func() (int64, error) {
			return q.EraseDeleteInvitations(ctx, db.EraseDeleteInvitationsParams{UserID: &id, Email: email})
		}},
		{"memberships", func() (int64, error) { return q.EraseDeleteMemberships(ctx, id) }},
		{"assignments", func() (int64, error) { return q.EraseDeleteAssignments(ctx, id) }},
		{"report_opt_outs", func() (int64, error) { return q.EraseDeleteReportOptOuts(ctx, id) }},
		{"outbox", func() (int64, error) { return q.EraseDeleteOutboxTo(ctx, email) }},
		{"report_recipients", func() (int64, error) { return q.EraseDeleteReportRecipient(ctx, email) }},
		{"reports_scrubbed", func() (int64, error) { return q.EraseScrubReportRecipients(ctx, email) }},
		{"ticket_requesters", func() (int64, error) { return q.EraseBlankTicketRequester(ctx, email) }},
		{"ticket_senders", func() (int64, error) { return q.EraseBlankMessageSender(ctx, email) }},
		{"inbound_emails", func() (int64, error) { return q.EraseDeleteInboundFrom(ctx, email) }},
		{"import_traces", func() (int64, error) {
			return q.EraseDeleteImportTracesOfUser(ctx, db.EraseDeleteImportTracesOfUserParams{UserID: id, Email: email})
		}},
	}

	for _, d := range steps {
		n, err := d.run()
		if err != nil {
			return nil, fmt.Errorf("%s : %w", d.name, err)
		}

		res.add(d.name, n)
	}

	kept, err := q.EraseCountTimeEntriesOfUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("compte du temps conserve : %w", err)
	}

	res.add("time_entries_kept", kept)

	// Le fichier de l'avatar part apres le COMMIT.
	if key := avatarKey(u.AvatarUrl); key != "" {
		res.keys = append(res.keys, key)
		res.add("avatars", 1)
	}

	// Compte du portail d'un client (detach) : ses exports propres sont aussi a
	// retirer. Effacement direct d'un compte : run() s'en charge pour le sujet,
	// inutile de les relire ici.
	if detach {
		exportKeys, err := q.EraseExportKeysOfSubject(ctx, db.EraseExportKeysOfSubjectParams{SubjectType: RgpdSubjectUser, SubjectID: id})
		if err != nil {
			return nil, fmt.Errorf("lecture des exports du compte : %w", err)
		}

		if len(exportKeys) > 0 {
			res.keys = append(res.keys, exportKeys...)
			res.add("exports", int64(len(exportKeys)))

			if _, err := q.EraseRedactExportsOfSubject(ctx, db.EraseRedactExportsOfSubjectParams{SubjectType: RgpdSubjectUser, SubjectID: id}); err != nil {
				return nil, fmt.Errorf("effacement du libelle des exports : %w", err)
			}
		}
	}

	if err := q.EraseAnonymizeUser(ctx, anonymize); err != nil {
		return nil, fmt.Errorf("anonymisation : %w", err)
	}

	if err := s.step("user"); err != nil {
		return nil, err
	}

	return res, nil
}

// eraseClient SUPPRIME un client et tout ce qui s'y rattache : comptes du
// portail (anonymises puis detaches), e-mails recus, contacts, projets avec
// leurs taches, tickets, livrables, rapports, temps et fichiers.
func (s *RgpdErasureService) eraseClient(ctx context.Context, tx pgx.Tx, q *db.Queries, clientID uuid.UUID) (*erasure, error) {
	res := &erasure{counts: map[string]int64{}}

	if _, err := q.LockClientForErasure(ctx, clientID); err != nil {
		return nil, fmt.Errorf("verrou du client : %w", err)
	}

	users, err := q.ListClientUsersForErasure(ctx, &clientID)
	if err != nil {
		return nil, fmt.Errorf("lecture des comptes du portail : %w", err)
	}

	contactEmails, err := q.EraseClientContactEmails(ctx, &clientID)
	if err != nil {
		return nil, fmt.Errorf("lecture des adresses des contacts : %w", err)
	}

	emails := append([]string{}, contactEmails...)

	for _, u := range users {
		if u.DeletedAt == nil {
			emails = append(emails, strings.ToLower(u.Email))
		}
	}

	for _, u := range users {
		sub, err := s.eraseUser(ctx, tx, q, u.ID, true)
		if err != nil {
			return nil, err
		}

		res.merge(sub)
		res.add("accounts", 1)
	}

	// Les cles de fichiers sont relevees AVANT la suppression des lignes.
	// Meme requete que l'export : ce qui est archive est exactement ce qui est efface.
	nFiles, err := s.clientFileKeys(ctx, q, clientID, res)
	if err != nil {
		return nil, err
	}

	res.add("files", nFiles)

	if err := s.step("files"); err != nil {
		return nil, err
	}

	totals, err := q.EraseClientCounts(ctx, &clientID)
	if err != nil {
		return nil, fmt.Errorf("compteurs : %w", err)
	}

	res.add("contacts", totals.Contacts)
	res.add("projects", totals.Projects)
	res.add("time_entries", totals.TimeEntries)
	res.add("tickets", totals.Tickets)

	n, err := q.EraseDeleteClientImportTraces(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("traces d'import : %w", err)
	}

	res.add("import_keys", n)

	if n, err = q.EraseDeleteClientOutbox(ctx, db.EraseDeleteClientOutboxParams{Emails: emails, ClientID: clientID}); err != nil {
		return nil, fmt.Errorf("e-mails sortants : %w", err)
	}

	res.add("outbox", n)

	if n, err = q.EraseDeleteClientInbound(ctx, db.EraseDeleteClientInboundParams{ClientID: &clientID, Emails: emails}); err != nil {
		return nil, fmt.Errorf("e-mails entrants : %w", err)
	}

	res.add("inbound_emails", n)

	if _, err = q.EraseDeleteClientContacts(ctx, &clientID); err != nil {
		return nil, fmt.Errorf("contacts : %w", err)
	}

	if _, err = q.EraseDeleteClientProjects(ctx, clientID); err != nil {
		return nil, fmt.Errorf("projets : %w", err)
	}

	if err := s.step("projects"); err != nil {
		return nil, err
	}

	if n, err = q.EraseDeleteClient(ctx, clientID); err != nil {
		return nil, fmt.Errorf("client : %w", err)
	} else if n != 1 {
		return nil, fmt.Errorf("client : %d ligne(s) supprimee(s), 1 attendue", n)
	}

	return res, nil
}

// ScheduleInactive planifie l'effacement des comptes non administrateurs
// inactifs depuis RGPD_INACTIVE_AFTER (0 : rien). Rend le nombre de comptes
// planifies ; deux instances ne planifient jamais deux fois le meme compte
// (index unique), le perdant est ignore.
func (s *RgpdErasureService) ScheduleInactive(ctx context.Context) (int, error) {
	if s.inactiveAfter <= 0 {
		return 0, nil
	}

	cutoff := s.now().Add(-s.inactiveAfter)
	total := 0

	for batch := 0; batch < rgpdInactiveMaxBatches && ctx.Err() == nil; batch++ {
		rows, err := s.q.ListInactiveUsers(ctx, db.ListInactiveUsersParams{Cutoff: cutoff, Batch: rgpdInactiveBatch})
		if err != nil {
			return total, fmt.Errorf("lecture des comptes inactifs : %w", err)
		}

		planned := 0

		for _, r := range rows {
			_, err := s.request(ctx, nil, RgpdSubjectUser, r.ID, RgpdReasonInactivity)

			var de *domain.Error

			switch {
			case err == nil:
				planned++
			case errors.As(err, &de):
				// Deja planifie ailleurs, ou disparu entre-temps : rien a faire.
			default:
				s.log.Warn("planification d'un effacement pour inactivite", "user_id", r.ID, "error", err)
			}
		}

		total += planned

		if planned == 0 || len(rows) < rgpdInactiveBatch {
			break
		}
	}

	return total, nil
}

// clientFileKeys releve les cles de tous les fichiers d'un client, page par
// page, avec la requete de l'export (RgpdClientFiles).
func (s *RgpdErasureService) clientFileKeys(ctx context.Context, q *db.Queries, clientID uuid.UUID, res *erasure) (int64, error) {
	var (
		total int64
		after uuid.UUID
	)

	for {
		rows, err := q.RgpdClientFiles(ctx, db.RgpdClientFilesParams{Subject: clientID, After: after, PageSize: rgpdPageSize})
		if err != nil {
			return total, fmt.Errorf("lecture des fichiers : %w", err)
		}

		for _, r := range rows {
			res.keys = append(res.keys, r.StorageKey)
			after = r.ID
			total++
		}

		if len(rows) < rgpdPageSize {
			return total, nil
		}
	}
}
