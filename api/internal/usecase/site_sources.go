package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// SiteSourcesService porte ce que Piilot surveille d'un site sans y toucher :
// ses sauvegardes, qui se signalent elles-memes, et ses domaines, releves par
// une tache de fond. Le silence d'une source compte comme une panne : c'est
// l'absence de signal qui declenche l'alerte, pas seulement l'echec annonce.
type SiteSourcesService struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	canSend  bool
	expiry   DomainExpiryReader
	hookBase string
	log      *slog.Logger
	now      func() time.Time
}

// DomainExpiryReader lit la date d'expiration d'un domaine.
type DomainExpiryReader interface {
	Expiry(ctx context.Context, domain string) (time.Time, error)
}

// NewSiteSourcesService construit le service. hookBase est l'adresse de
// l'application, d'ou se deduit le point d'appel des sauvegardes.
func NewSiteSourcesService(pool *pgxpool.Pool, expiry DomainExpiryReader, hookBase string, canSend bool, log *slog.Logger) *SiteSourcesService {
	return &SiteSourcesService{pool: pool, q: db.New(pool), canSend: canSend, expiry: expiry, hookBase: strings.TrimRight(hookBase, "/"), log: log, now: time.Now}
}

// ErrSiteBackupForbidden : le jeton du point d'appel ne correspond a rien.
var ErrSiteBackupForbidden = &domain.Error{Status: http.StatusUnauthorized, Code: "INVALID_TOKEN", Message: "Jeton de sauvegarde invalide"}

const (
	// siteBackupRetention couvre l'archive des rapports de l'annee.
	siteBackupRetention = 400 * 24 * time.Hour
	maxBackupMessage    = 500
	maxBackupAge        = 7 * 24 * time.Hour
	maxProjectDomains   = 10
)

// --- Vue du projet --------------------------------------------------------------

// SiteBackupRun est une execution signalee.
type SiteBackupRun struct {
	Status     string    `json:"status"`
	SizeBytes  *int64    `json:"size_bytes"`
	Message    string    `json:"message"`
	FinishedAt time.Time `json:"finished_at"`
}

// SiteBackupView est la surveillance des sauvegardes d'un projet.
type SiteBackupView struct {
	ExpectedHours     int             `json:"expected_hours"`
	LastSuccessAt     *time.Time      `json:"last_success_at"`
	LastFailureAt     *time.Time      `json:"last_failure_at"`
	LastRestoreTestOn *string         `json:"last_restore_test_on"`
	Healthy           bool            `json:"healthy"`
	Runs              []SiteBackupRun `json:"runs"`
}

// SiteDomainView est un domaine d'un projet.
type SiteDomainView struct {
	ID        uuid.UUID  `json:"id"`
	Domain    string     `json:"domain"`
	ExpiresAt *time.Time `json:"expires_at"`
	DaysLeft  *int       `json:"days_left"`
	CheckedAt *time.Time `json:"checked_at"`
	Failing   bool       `json:"failing"`
}

// SiteSourcesView est la carte « Sauvegardes et domaines » d'un projet.
type SiteSourcesView struct {
	HookURL string           `json:"hook_url"`
	Backup  *SiteBackupView  `json:"backup"`
	Domains []SiteDomainView `json:"domains"`
}

func (s *SiteSourcesService) requireProject(ctx context.Context, projectID uuid.UUID) error {
	exists, err := s.q.ReportProjectExists(ctx, projectID)
	if err != nil {
		return fmt.Errorf("lecture du projet : %w", err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return nil
}

func daysLeft(at, now time.Time) int {
	return int(at.Sub(now).Hours() / 24)
}

func backupHealthy(row db.SiteBackupSource, now time.Time) bool {
	if row.LastSuccessAt == nil {
		return false
	}
	if row.LastFailureAt != nil && row.LastFailureAt.After(*row.LastSuccessAt) {
		return false
	}
	return now.Sub(*row.LastSuccessAt) <= time.Duration(row.ExpectedHours)*time.Hour
}

// View rend la carte d'un projet.
func (s *SiteSourcesService) View(ctx context.Context, projectID uuid.UUID) (SiteSourcesView, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return SiteSourcesView{}, err
	}
	now := s.now()
	out := SiteSourcesView{HookURL: s.hookBase + "/api/v1/hooks/site-backups", Domains: []SiteDomainView{}}

	row, err := s.q.GetSiteBackupSource(ctx, projectID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return SiteSourcesView{}, fmt.Errorf("source des sauvegardes : %w", err)
	default:
		runs, err := s.q.ListSiteBackupRuns(ctx, projectID)
		if err != nil {
			return SiteSourcesView{}, fmt.Errorf("sauvegardes du projet : %w", err)
		}
		view := &SiteBackupView{
			ExpectedHours: int(row.ExpectedHours), LastSuccessAt: row.LastSuccessAt, LastFailureAt: row.LastFailureAt,
			Healthy: backupHealthy(row, now), Runs: make([]SiteBackupRun, 0, len(runs)),
		}
		if row.LastRestoreTestOn != nil {
			day := row.LastRestoreTestOn.Format(dateLayout)
			view.LastRestoreTestOn = &day
		}
		for _, r := range runs {
			view.Runs = append(view.Runs, SiteBackupRun{Status: r.Status, SizeBytes: r.SizeBytes, Message: r.Message, FinishedAt: r.FinishedAt})
		}
		out.Backup = view
	}

	domains, err := s.q.ListProjectDomains(ctx, projectID)
	if err != nil {
		return SiteSourcesView{}, fmt.Errorf("domaines du projet : %w", err)
	}
	for _, d := range domains {
		item := SiteDomainView{ID: d.ID, Domain: d.Domain, ExpiresAt: d.ExpiresAt, CheckedAt: d.CheckedAt, Failing: d.LastError != ""}
		if d.ExpiresAt != nil {
			left := daysLeft(*d.ExpiresAt, now)
			item.DaysLeft = &left
		}
		out.Domains = append(out.Domains, item)
	}
	return out, nil
}

// --- Sauvegardes ----------------------------------------------------------------

// SiteBackupToken est le jeton tout juste cree : il ne se revoit jamais.
type SiteBackupToken struct {
	Token   string `json:"token"`
	HookURL string `json:"hook_url"`
}

// RotateBackupToken cree la source si besoin et tire un nouveau jeton. L'ancien
// cesse aussitot de fonctionner.
func (s *SiteSourcesService) RotateBackupToken(ctx context.Context, projectID uuid.UUID) (SiteBackupToken, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return SiteBackupToken{}, err
	}
	raw, hash, err := security.NewOneTimeToken()
	if err != nil {
		return SiteBackupToken{}, fmt.Errorf("tirage du jeton : %w", err)
	}
	if err := s.q.UpsertSiteBackupToken(ctx, db.UpsertSiteBackupTokenParams{ProjectID: projectID, TokenHash: hash}); err != nil {
		return SiteBackupToken{}, fmt.Errorf("enregistrement du jeton : %w", err)
	}
	return SiteBackupToken{Token: raw, HookURL: s.hookBase + "/api/v1/hooks/site-backups"}, nil
}

// UpdateBackup enregistre le delai attendu et le dernier test de restauration.
func (s *SiteSourcesService) UpdateBackup(ctx context.Context, projectID uuid.UUID, expectedHours int, restoreTestOn *time.Time) (SiteSourcesView, error) {
	details := map[string]any{}
	if expectedHours < 1 || expectedHours > 744 {
		details["expected_hours"] = "Entre 1 et 744 heures"
	}
	if restoreTestOn != nil && restoreTestOn.After(s.now()) {
		details["last_restore_test_on"] = "Un test de restauration ne peut pas être à venir"
	}
	if len(details) > 0 {
		return SiteSourcesView{}, domain.ErrValidation.WithDetails(details)
	}
	n, err := s.q.UpdateSiteBackupSource(ctx, db.UpdateSiteBackupSourceParams{
		ProjectID: projectID, ExpectedHours: int32(expectedHours), LastRestoreTestOn: restoreTestOn,
	})
	if err != nil {
		return SiteSourcesView{}, fmt.Errorf("enregistrement des sauvegardes : %w", err)
	}
	if n == 0 {
		return SiteSourcesView{}, domain.ErrNotFound
	}
	return s.View(ctx, projectID)
}

// RemoveBackup cesse de surveiller les sauvegardes d'un projet. Son historique
// reste : les rapports deja generes en parlent.
func (s *SiteSourcesService) RemoveBackup(ctx context.Context, projectID uuid.UUID) (SiteSourcesView, error) {
	n, err := s.q.DeleteSiteBackupSource(ctx, projectID)
	if err != nil {
		return SiteSourcesView{}, fmt.Errorf("retrait des sauvegardes : %w", err)
	}
	if n == 0 {
		return SiteSourcesView{}, domain.ErrNotFound
	}
	return s.View(ctx, projectID)
}

// SiteBackupReport est ce que le script de sauvegarde signale.
type SiteBackupReport struct {
	Status     string
	SizeBytes  *int64
	Message    string
	FinishedAt *time.Time
}

// RecordBackup enregistre une execution signalee par le point d'appel.
func (s *SiteSourcesService) RecordBackup(ctx context.Context, token string, in SiteBackupReport) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrSiteBackupForbidden
	}
	projectID, err := s.q.FindSiteBackupProject(ctx, security.HashOneTimeToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSiteBackupForbidden
	}
	if err != nil {
		return fmt.Errorf("lecture du jeton : %w", err)
	}

	now := s.now()
	at := now
	if in.FinishedAt != nil {
		at = *in.FinishedAt
	}
	details := map[string]any{}
	if in.Status != "success" && in.Status != "failed" {
		details["status"] = "success ou failed"
	}
	if in.SizeBytes != nil && *in.SizeBytes < 0 {
		details["size_bytes"] = "Une taille positive"
	}
	if now.Sub(at) > maxBackupAge {
		details["finished_at"] = "Trop ancienne"
	}
	if len(details) > 0 {
		return domain.ErrValidation.WithDetails(details)
	}
	// Une horloge en avance chez le site ne doit pas masquer les jours suivants.
	if at.After(now) {
		at = now
	}
	message := []rune(strings.TrimSpace(in.Message))
	if len(message) > maxBackupMessage {
		message = message[:maxBackupMessage]
	}

	if err := s.q.InsertSiteBackupRun(ctx, db.InsertSiteBackupRunParams{
		ProjectID: projectID, Status: in.Status, SizeBytes: in.SizeBytes, Message: string(message), FinishedAt: at,
	}); err != nil {
		return fmt.Errorf("enregistrement de la sauvegarde : %w", err)
	}
	if in.Status == "success" {
		err = s.q.RecordSiteBackupSuccess(ctx, db.RecordSiteBackupSuccessParams{ProjectID: projectID, At: at})
	} else {
		err = s.q.RecordSiteBackupFailure(ctx, db.RecordSiteBackupFailureParams{ProjectID: projectID, At: at})
	}
	if err != nil {
		return fmt.Errorf("etat des sauvegardes : %w", err)
	}
	return nil
}

// --- Domaines -------------------------------------------------------------------

var domainName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,61}[a-z0-9]$`)

// cleanDomain ramene une saisie a un nom de domaine, adresse collee comprise.
func cleanDomain(raw string) (string, bool) {
	d := strings.ToLower(strings.TrimSpace(raw))
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	d = strings.TrimSuffix(strings.TrimPrefix(d, "www."), ".")
	return d, len(d) <= 253 && domainName.MatchString(d)
}

// AddDomain suit un domaine de plus. La date arrive au prochain passage de la
// tache : aucun appel exterieur pendant une requete.
func (s *SiteSourcesService) AddDomain(ctx context.Context, projectID uuid.UUID, raw string) (SiteSourcesView, error) {
	if err := s.requireProject(ctx, projectID); err != nil {
		return SiteSourcesView{}, err
	}
	name, ok := cleanDomain(raw)
	if !ok {
		return SiteSourcesView{}, domain.ErrValidation.WithDetails(map[string]any{"domain": "Un nom de domaine, par exemple exemple.fr"})
	}
	count, err := s.q.CountProjectDomains(ctx, projectID)
	if err != nil {
		return SiteSourcesView{}, fmt.Errorf("domaines du projet : %w", err)
	}
	if count >= maxProjectDomains {
		return SiteSourcesView{}, domain.ErrValidation.WithDetails(map[string]any{"domain": "10 domaines au plus"})
	}
	_, err = s.q.InsertProjectDomain(ctx, db.InsertProjectDomainParams{ProjectID: projectID, Domain: name})
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteSourcesView{}, domain.ErrValidation.WithDetails(map[string]any{"domain": "Ce domaine est déjà suivi"})
	}
	if err != nil {
		return SiteSourcesView{}, fmt.Errorf("ajout du domaine : %w", err)
	}
	return s.View(ctx, projectID)
}

// RemoveDomain cesse de suivre un domaine.
func (s *SiteSourcesService) RemoveDomain(ctx context.Context, projectID, id uuid.UUID) (SiteSourcesView, error) {
	n, err := s.q.DeleteProjectDomain(ctx, db.DeleteProjectDomainParams{ID: id, ProjectID: projectID})
	if err != nil {
		return SiteSourcesView{}, fmt.Errorf("retrait du domaine : %w", err)
	}
	if n == 0 {
		return SiteSourcesView{}, domain.ErrNotFound
	}
	return s.View(ctx, projectID)
}

// --- Tache de fond --------------------------------------------------------------

// Tick est le passage de la tache. Une etape en echec n'empeche pas les autres.
func (s *SiteSourcesService) Tick(ctx context.Context) error {
	now := s.now()
	return errors.Join(
		s.watchBackups(ctx, now),
		s.checkDomains(ctx, now),
		s.alertDomains(ctx, now),
		s.q.PruneSiteBackupRuns(ctx, now.Add(-siteBackupRetention)),
	)
}

// alertAndMark ecrit l'alerte et son marquage ensemble : une coupure entre les
// deux referait partir l'alerte a chacun, ou la perdrait.
func (s *SiteSourcesService) alertAndMark(ctx context.Context, a siteAlert, mark func(q *db.Queries) error) error {
	return inTx(ctx, s.pool, s.q, func(q *db.Queries) error {
		if err := sendSiteAlert(ctx, q, s.canSend, s.hookBase, a); err != nil {
			return err
		}
		return mark(q)
	})
}

func (s *SiteSourcesService) watchBackups(ctx context.Context, now time.Time) error {
	rows, err := s.q.ListSiteBackupsToAlert(ctx, now)
	if err != nil {
		return fmt.Errorf("sauvegardes a signaler : %w", err)
	}
	var errs []error
	for _, r := range rows {
		failed := r.LastFailureAt != nil && (r.LastSuccessAt == nil || r.LastFailureAt.After(*r.LastSuccessAt))
		subject, message := "Sauvegardes muettes · "+r.ProjectName, fmt.Sprintf("Aucune sauvegarde récente pour %s : le site ne les signale plus dans le délai attendu.", r.ProjectName)
		if failed {
			subject, message = "Sauvegarde en échec · "+r.ProjectName, fmt.Sprintf("La dernière sauvegarde de %s a échoué.", r.ProjectName)
		}
		if err := s.alertAndMark(ctx, siteAlert{
			Kind: "site_backup_stale", ProjectID: r.ProjectID, Subject: subject, Message: message,
			Payload: map[string]any{"project_name": r.ProjectName, "last_success_at": r.LastSuccessAt, "failed": failed},
		}, func(q *db.Queries) error {
			return q.MarkSiteBackupAlerted(ctx, db.MarkSiteBackupAlertedParams{ProjectID: r.ProjectID, At: &now})
		}); err != nil {
			errs = append(errs, fmt.Errorf("alerte des sauvegardes : %w", err))
		}
	}
	return errors.Join(errs...)
}

func (s *SiteSourcesService) checkDomains(ctx context.Context, now time.Time) error {
	rows, err := s.q.ListDomainsToCheck(ctx, now)
	if err != nil {
		return fmt.Errorf("domaines a relever : %w", err)
	}
	var errs []error
	for _, d := range rows {
		at, err := s.expiry.Expiry(ctx, d.Domain)
		if err != nil {
			s.log.Warn("releve du domaine", "domain", d.Domain, "error", err)
			if err := s.q.RecordDomainError(ctx, db.RecordDomainErrorParams{ID: d.ID, At: &now, LastError: err.Error()}); err != nil {
				errs = append(errs, fmt.Errorf("erreur du domaine : %w", err))
			}
			continue
		}
		if err := s.q.RecordDomainExpiry(ctx, db.RecordDomainExpiryParams{ID: d.ID, ExpiresAt: &at, At: &now}); err != nil {
			errs = append(errs, fmt.Errorf("date du domaine : %w", err))
		}
	}
	return errors.Join(errs...)
}

// domainAlertSteps : les paliers d'alerte, du plus lointain au plus proche.
var domainAlertSteps = []int{60, 30, 14, 7, 1}

// domainAlertStep rend le palier atteint a N jours de l'expiration, et s'il
// reste a annoncer. Sauter un palier n'en annonce qu'un : le plus proche.
func domainAlertStep(days int, last *int32) (int, bool) {
	step := 0
	for _, s := range domainAlertSteps {
		if days <= s {
			step = s
		}
	}
	if step == 0 {
		return 0, false
	}
	return step, last == nil || int(*last) > step
}

func domainExpiryMessage(domainName, project string, left int) string {
	if left <= 0 {
		return fmt.Sprintf("Le domaine %s de %s a expiré, ou expire aujourd’hui.", domainName, project)
	}
	return fmt.Sprintf("Le domaine %s de %s expire dans %d jour(s).", domainName, project, left)
}

func (s *SiteSourcesService) alertDomains(ctx context.Context, now time.Time) error {
	rows, err := s.q.ListDomainsExpiring(ctx, now)
	if err != nil {
		return fmt.Errorf("domaines qui expirent : %w", err)
	}
	var errs []error
	for _, d := range rows {
		if d.ExpiresAt == nil {
			continue
		}
		left := daysLeft(*d.ExpiresAt, now)
		step, due := domainAlertStep(left, d.LastAlertDays)
		// Renouvele : les paliers repartent pour la prochaine echeance.
		if step == 0 {
			if d.LastAlertDays != nil {
				if err := s.q.SetDomainAlertDays(ctx, db.SetDomainAlertDaysParams{ID: d.ID}); err != nil {
					errs = append(errs, fmt.Errorf("paliers du domaine : %w", err))
				}
			}
			continue
		}
		if !due {
			continue
		}
		reached := int32(step)
		if err := s.alertAndMark(ctx, siteAlert{
			Kind: "domain_expiring", ProjectID: d.ProjectID,
			Subject: "Domaine à renouveler · " + d.Domain, Message: domainExpiryMessage(d.Domain, d.ProjectName, left),
			Payload: map[string]any{"project_name": d.ProjectName, "domain": d.Domain, "days_left": left},
		}, func(q *db.Queries) error {
			return q.SetDomainAlertDays(ctx, db.SetDomainAlertDaysParams{ID: d.ID, LastAlertDays: &reached})
		}); err != nil {
			errs = append(errs, fmt.Errorf("alerte du domaine : %w", err))
		}
	}

	failing, err := s.q.ListDomainsFailing(ctx, now)
	if err != nil {
		return errors.Join(append(errs, fmt.Errorf("domaines en erreur : %w", err))...)
	}
	for _, d := range failing {
		if err := s.alertAndMark(ctx, siteAlert{
			Kind: "domain_check_failed", ProjectID: d.ProjectID,
			Subject: "Échéance de domaine inconnue · " + d.Domain,
			Message: fmt.Sprintf("L’échéance du domaine %s de %s ne se relève plus : une expiration passerait inaperçue. Vérifiez-la chez le registraire.", d.Domain, d.ProjectName),
			Payload: map[string]any{"project_name": d.ProjectName, "domain": d.Domain},
		}, func(q *db.Queries) error {
			return q.MarkDomainFailureAlerted(ctx, db.MarkDomainFailureAlertedParams{ID: d.ID, At: &now})
		}); err != nil {
			errs = append(errs, fmt.Errorf("alerte du domaine : %w", err))
		}
	}
	return errors.Join(errs...)
}
