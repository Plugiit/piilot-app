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
	"github.com/plugiit/piilot-app/api/internal/githubactions"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// WorkflowDispatcher demande le lancement d'un workflow.
type WorkflowDispatcher interface {
	Dispatch(ctx context.Context, d githubactions.Dispatch) error
}

// SiteUpdateService commande les mises a jour des sites, sur l'environnement
// choisi pour chaque projet. Il n'execute rien : la CI du depot fait le travail
// et rend compte. Un lancement sans compte rendu compte comme un echec, et un
// echec arrete la planification jusqu'a ce qu'un humain relance.
type SiteUpdateService struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	canSend  bool
	github   WorkflowDispatcher
	sealKey  []byte
	hookBase string
	log      *slog.Logger
	now      func() time.Time
}

// NewSiteUpdateService construit le service.
func NewSiteUpdateService(pool *pgxpool.Pool, github WorkflowDispatcher, secret []byte, hookBase string, canSend bool, log *slog.Logger) *SiteUpdateService {
	return &SiteUpdateService{
		pool: pool, q: db.New(pool), canSend: canSend, github: github, sealKey: security.DeriveKey(secret, "site-update-github-token"),
		hookBase: strings.TrimRight(hookBase, "/"), log: log, now: time.Now,
	}
}

// ErrSiteUpdateForbidden : le jeton du compte rendu ne correspond a aucun
// lancement en cours.
var ErrSiteUpdateForbidden = &domain.Error{Status: http.StatusUnauthorized, Code: "INVALID_TOKEN", Message: "Jeton de mise à jour invalide"}

const (
	maxDispatchAttempts = 3
	dispatchRetryDelay  = 5 * time.Minute
	// dispatchingTimeout : au-dela, l'instance est tombee pendant l'appel.
	dispatchingTimeout = 10 * time.Minute
	// callbackTimeout : le temps laisse a la CI pour rendre compte.
	callbackTimeout = 2 * time.Hour
	// callbackMaxAge borne un jeton lisible dans les entrees du workflow : passe
	// ce delai, il ne peut plus reecrire un vieux lancement.
	callbackMaxAge    = 7 * 24 * time.Hour
	maxRunMessage     = 500
	siteUpdateHookURL = "/api/v1/hooks/site-updates"
)

var (
	workflowName = regexp.MustCompile(`^[A-Za-z0-9._-]+\.ya?ml$`)
	gitRef       = regexp.MustCompile(`^[A-Za-z0-9._/-]{1,200}$`)
)

// githubRepoOf rend le proprietaire et le nom d'un depot GitHub. GitLab n'est
// pas pris en charge ici : il ne connait pas workflow_dispatch.
func githubRepoOf(repoURL string) (owner, repo string, ok bool) {
	rest, found := strings.CutPrefix(NormalizeRepoURL(repoURL), "https://github.com/")
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func validRef(ref string) bool {
	return gitRef.MatchString(ref) && !strings.Contains(ref, "..") && !strings.HasPrefix(ref, "/") && !strings.HasSuffix(ref, "/")
}

// --- Jeton de l'agence ----------------------------------------------------------

// SiteUpdateTokenStatus dit si un jeton est enregistre. Il ne se relit jamais.
type SiteUpdateTokenStatus struct {
	Configured bool       `json:"configured"`
	UpdatedAt  *time.Time `json:"updated_at"`
}

// TokenStatus rend l'etat du jeton.
func (s *SiteUpdateService) TokenStatus(ctx context.Context) (SiteUpdateTokenStatus, error) {
	row, err := s.q.GetSiteUpdateToken(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteUpdateTokenStatus{}, nil
	}
	if err != nil {
		return SiteUpdateTokenStatus{}, fmt.Errorf("lecture du jeton : %w", err)
	}
	return SiteUpdateTokenStatus{Configured: true, UpdatedAt: &row.UpdatedAt}, nil
}

// SetToken enregistre le jeton GitHub, chiffre.
func (s *SiteUpdateService) SetToken(ctx context.Context, token string) (SiteUpdateTokenStatus, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 1000 || strings.ContainsAny(token, " \t\r\n") {
		return SiteUpdateTokenStatus{}, domain.ErrValidation.WithDetails(map[string]any{"token": "Un jeton GitHub"})
	}
	sealed, err := security.Seal(s.sealKey, token)
	if err != nil {
		return SiteUpdateTokenStatus{}, fmt.Errorf("chiffrement du jeton : %w", err)
	}
	if err := s.q.SetSiteUpdateToken(ctx, sealed); err != nil {
		return SiteUpdateTokenStatus{}, fmt.Errorf("enregistrement du jeton : %w", err)
	}
	return s.TokenStatus(ctx)
}

// ClearToken retire le jeton.
func (s *SiteUpdateService) ClearToken(ctx context.Context) (SiteUpdateTokenStatus, error) {
	if err := s.q.ClearSiteUpdateToken(ctx); err != nil {
		return SiteUpdateTokenStatus{}, fmt.Errorf("retrait du jeton : %w", err)
	}
	return SiteUpdateTokenStatus{}, nil
}

// --- Vue du projet --------------------------------------------------------------

// SiteUpdateTarget est ce qui a ete choisi pour un projet.
type SiteUpdateTarget struct {
	Environment string `json:"environment"`
	Workflow    string `json:"workflow"`
	Ref         string `json:"ref"`
	EveryDays   *int   `json:"every_days"`
	Enabled     bool   `json:"enabled"`
}

// SiteUpdateRun est un lancement.
type SiteUpdateRun struct {
	ID           uuid.UUID  `json:"id"`
	Environment  string     `json:"environment"`
	Workflow     string     `json:"workflow"`
	Ref          string     `json:"ref"`
	Status       string     `json:"status"`
	Trigger      string     `json:"trigger"`
	Message      string     `json:"message"`
	RunURL       string     `json:"run_url"`
	RequestedAt  time.Time  `json:"requested_at"`
	DispatchedAt *time.Time `json:"dispatched_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

// SiteUpdatesView est la carte « Mises a jour » d'un projet.
type SiteUpdatesView struct {
	Repo          string `json:"repo"`
	RepoSupported bool   `json:"repo_supported"`
	// Le depot du projet n'est plus celui confirme avec la cible.
	RepoChanged     bool              `json:"repo_changed"`
	TokenConfigured bool              `json:"token_configured"`
	HookURL         string            `json:"hook_url"`
	Target          *SiteUpdateTarget `json:"target"`
	Runs            []SiteUpdateRun   `json:"runs"`
}

// View rend la carte d'un projet.
func (s *SiteUpdateService) View(ctx context.Context, projectID uuid.UUID) (SiteUpdatesView, error) {
	repo, err := s.q.GetProjectRepoURL(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteUpdatesView{}, domain.ErrNotFound
	}
	if err != nil {
		return SiteUpdatesView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	token, err := s.TokenStatus(ctx)
	if err != nil {
		return SiteUpdatesView{}, err
	}
	_, _, supported := githubRepoOf(repo)
	out := SiteUpdatesView{Repo: repo, RepoSupported: supported, TokenConfigured: token.Configured, HookURL: s.hookBase + siteUpdateHookURL, Runs: []SiteUpdateRun{}}

	target, err := s.q.GetSiteUpdateTarget(ctx, projectID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return SiteUpdatesView{}, fmt.Errorf("cible des mises a jour : %w", err)
	default:
		view := &SiteUpdateTarget{Environment: target.Environment, Workflow: target.Workflow, Ref: target.Ref, Enabled: target.Enabled}
		if target.EveryDays != nil {
			days := int(*target.EveryDays)
			view.EveryDays = &days
		}
		out.Target = view
		out.RepoChanged = target.RepoUrl != repo
	}

	runs, err := s.q.ListSiteUpdateRuns(ctx, projectID)
	if err != nil {
		return SiteUpdatesView{}, fmt.Errorf("lancements du projet : %w", err)
	}
	for _, r := range runs {
		out.Runs = append(out.Runs, SiteUpdateRun{
			ID: r.ID, Environment: r.Environment, Workflow: r.Workflow, Ref: r.Ref, Status: r.Status, Trigger: r.Trigger,
			Message: r.Message, RunURL: r.RunUrl, RequestedAt: r.RequestedAt, DispatchedAt: r.DispatchedAt, FinishedAt: r.FinishedAt,
		})
	}
	return out, nil
}

// SiteUpdateTargetInput est ce que l'ecran enregistre.
type SiteUpdateTargetInput struct {
	Environment, Workflow, Ref string
	EveryDays                  *int
	Enabled                    bool
}

// SetTarget enregistre l'environnement choisi. Un lancement deja demande garde
// ce qui avait ete demande.
func (s *SiteUpdateService) SetTarget(ctx context.Context, projectID uuid.UUID, in SiteUpdateTargetInput) (SiteUpdatesView, error) {
	details := map[string]any{}
	if in.Environment != "prod" && in.Environment != "preprod" {
		details["environment"] = "Choisissez un environnement"
	}
	if !workflowName.MatchString(in.Workflow) {
		details["workflow"] = "Le nom d’un fichier de .github/workflows, par exemple piilot-update.yml"
	}
	if !validRef(in.Ref) {
		details["ref"] = "Le nom d’une branche, par exemple main"
	}
	if in.EveryDays != nil && (*in.EveryDays < 1 || *in.EveryDays > 90) {
		details["every_days"] = "Entre 1 et 90 jours"
	}
	if len(details) > 0 {
		return SiteUpdatesView{}, domain.ErrValidation.WithDetails(details)
	}
	repo, err := s.q.GetProjectRepoURL(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteUpdatesView{}, domain.ErrNotFound
	}
	if err != nil {
		return SiteUpdatesView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	var every *int32
	if in.EveryDays != nil {
		days := int32(*in.EveryDays)
		every = &days
	}
	if err := s.q.UpsertSiteUpdateTarget(ctx, db.UpsertSiteUpdateTargetParams{
		ProjectID: projectID, Environment: in.Environment, Workflow: in.Workflow, Ref: in.Ref, RepoUrl: repo, EveryDays: every, Enabled: in.Enabled,
	}); err != nil {
		return SiteUpdatesView{}, fmt.Errorf("enregistrement de la cible : %w", err)
	}
	return s.View(ctx, projectID)
}

// RequestRun demande un lancement. L'environnement est redit par l'ecran : un
// onglet resté ouvert sur l'ancien choix ne lance rien sur le nouveau.
func (s *SiteUpdateService) RequestRun(ctx context.Context, projectID, userID uuid.UUID, confirmedEnvironment string) (SiteUpdatesView, error) {
	target, err := s.q.GetSiteUpdateTarget(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteUpdatesView{}, domain.ErrConflict.WithMessage("Choisissez d’abord l’environnement des mises à jour")
	}
	if err != nil {
		return SiteUpdatesView{}, fmt.Errorf("cible des mises a jour : %w", err)
	}
	if !target.Enabled {
		return SiteUpdatesView{}, domain.ErrConflict.WithMessage("Les mises à jour de ce projet sont désactivées")
	}
	if target.Environment != confirmedEnvironment {
		return SiteUpdatesView{}, domain.ErrConflict.WithMessage("L’environnement a changé entre-temps : rechargez la page")
	}
	repo, err := s.q.GetProjectRepoURL(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SiteUpdatesView{}, domain.ErrNotFound
	}
	if err != nil {
		return SiteUpdatesView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if _, _, ok := githubRepoOf(repo); !ok {
		return SiteUpdatesView{}, domain.ErrConflict.WithMessage("Ce projet n’a pas de dépôt GitHub")
	}
	if repo != target.RepoUrl {
		return SiteUpdatesView{}, domain.ErrConflict.WithMessage("Le dépôt du projet a changé : enregistrez à nouveau la cible pour le confirmer")
	}
	token, err := s.TokenStatus(ctx)
	if err != nil {
		return SiteUpdatesView{}, err
	}
	if !token.Configured {
		return SiteUpdatesView{}, domain.ErrConflict.WithMessage("Aucun jeton GitHub : renseignez-le dans Paramètres › Dépôts Git")
	}
	if err := s.enqueue(ctx, projectID, target.Environment, target.Workflow, target.Ref, target.RepoUrl, "manual", &userID); err != nil {
		if isUniqueViolation(err) {
			return SiteUpdatesView{}, domain.ErrConflict.WithMessage("Une mise à jour est déjà en cours sur ce projet")
		}
		return SiteUpdatesView{}, err
	}
	return s.View(ctx, projectID)
}

// inTx : un etat et son alerte s'ecrivent ensemble. Sinon, une coupure entre les
// deux laisse un echec que personne n'apprend jamais.
func (s *SiteUpdateService) inTx(ctx context.Context, run func(q *db.Queries) error) error {
	return inTx(ctx, s.pool, s.q, run)
}

func (s *SiteUpdateService) enqueue(ctx context.Context, projectID uuid.UUID, environment, workflow, ref, repoURL, trigger string, userID *uuid.UUID) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.InsertSiteUpdateRun(ctx, db.InsertSiteUpdateRunParams{
			ProjectID: projectID, Environment: environment, Workflow: workflow, Ref: ref, RepoUrl: repoURL, Trigger: trigger, RequestedBy: userID,
		}); err != nil {
			return fmt.Errorf("demande de lancement : %w", err)
		}
		now := s.now()
		if err := q.TouchSiteUpdateTarget(ctx, db.TouchSiteUpdateTargetParams{ProjectID: projectID, At: &now}); err != nil {
			return fmt.Errorf("demande de lancement : %w", err)
		}
		return nil
	})
}

// --- Compte rendu de la CI ------------------------------------------------------

// SiteUpdateReport est ce que la CI rend.
type SiteUpdateReport struct {
	Status, Message, RunURL string
}

// RecordResult enregistre le compte rendu d'un lancement.
func (s *SiteUpdateService) RecordResult(ctx context.Context, token string, in SiteUpdateReport) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrSiteUpdateForbidden
	}
	run, err := s.q.FindSiteUpdateRunByToken(ctx, db.FindSiteUpdateRunByTokenParams{CallbackTokenHash: security.HashOneTimeToken(token), Since: s.now().Add(-callbackMaxAge)})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSiteUpdateForbidden
	}
	if err != nil {
		return fmt.Errorf("lecture du jeton : %w", err)
	}
	if in.Status != "success" && in.Status != "failed" {
		return domain.ErrValidation.WithDetails(map[string]any{"status": "success ou failed"})
	}
	// Seule une adresse de GitHub s'affiche comme un lien.
	runURL := strings.TrimSpace(in.RunURL)
	if !strings.HasPrefix(runURL, "https://github.com/") {
		runURL = ""
	}
	now := s.now()
	message := cutMessage(in.Message)
	// En cas d'erreur, tout est annule et le jeton reste bon : le nouvel essai de
	// la CI aboutira.
	return s.inTx(ctx, func(q *db.Queries) error {
		n, err := q.FinishSiteUpdateRun(ctx, db.FinishSiteUpdateRunParams{ID: run.ID, Status: in.Status, Message: message, RunUrl: runURL, At: &now})
		if err != nil {
			return fmt.Errorf("compte rendu du lancement : %w", err)
		}
		if n == 0 {
			return ErrSiteUpdateForbidden
		}
		if in.Status == "failed" {
			return s.alertFailure(ctx, q, run.ProjectID, run.ProjectName, run.Environment, message)
		}
		return nil
	})
}

func cutMessage(raw string) string {
	message := []rune(strings.TrimSpace(raw))
	if len(message) > maxRunMessage {
		message = message[:maxRunMessage]
	}
	return string(message)
}

// --- Tache de fond --------------------------------------------------------------

// Tick est le passage de la tache. Une etape en echec n'empeche pas les autres.
func (s *SiteUpdateService) Tick(ctx context.Context) error {
	return s.TickAt(ctx, s.now())
}

// TickAt est le passage de la tache a un instant donne.
func (s *SiteUpdateService) TickAt(ctx context.Context, now time.Time) error {
	return errors.Join(s.schedule(ctx, now), s.dispatchDue(ctx, now), s.sweepStuck(ctx, now))
}

func (s *SiteUpdateService) schedule(ctx context.Context, now time.Time) error {
	due, err := s.q.ListSiteUpdatesDue(ctx, now)
	if err != nil {
		return fmt.Errorf("mises a jour planifiees : %w", err)
	}
	var errs []error
	for _, t := range due {
		// Un lancement manuel en cours : la planification repassera.
		if err := s.enqueue(ctx, t.ProjectID, t.Environment, t.Workflow, t.Ref, t.RepoUrl, "schedule", nil); err != nil && !isUniqueViolation(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// dispatchBatch borne un passage : chaque lancement coute un appel a GitHub.
const dispatchBatch = 10

func (s *SiteUpdateService) dispatchDue(ctx context.Context, now time.Time) error {
	var errs []error
	for range dispatchBatch {
		raw, hash, err := security.NewOneTimeToken()
		if err != nil {
			return errors.Join(append(errs, fmt.Errorf("tirage du jeton : %w", err))...)
		}
		run, err := s.q.ClaimSiteUpdateRun(ctx, db.ClaimSiteUpdateRunParams{CallbackTokenHash: hash, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.Join(errs...)
		}
		if err != nil {
			return errors.Join(append(errs, fmt.Errorf("prochain lancement : %w", err))...)
		}
		if err := s.dispatch(ctx, run, raw, now); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SiteUpdateService) dispatch(ctx context.Context, run db.SiteUpdateRun, callbackToken string, now time.Time) error {
	// Une erreur de base ici n'est pas un diagnostic : le lancement reste en
	// « dispatching » et la surveillance le signale. GitHub n'a pas ete appele.
	project, err := s.q.GetSiteUpdateRunForDispatch(ctx, run.ProjectID)
	if err != nil {
		return fmt.Errorf("projet du lancement : %w", err)
	}
	fail := func(message string) error {
		return s.inTx(ctx, func(q *db.Queries) error {
			if err := q.FailSiteUpdateRun(ctx, db.FailSiteUpdateRunParams{ID: run.ID, Message: cutMessage(message), At: &now}); err != nil {
				return fmt.Errorf("echec du lancement : %w", err)
			}
			return s.alertFailure(ctx, q, run.ProjectID, project, run.Environment, message)
		})
	}

	// La cible a pu changer depuis la demande, en attendant un nouvel essai :
	// un admin qui desactive ou change d'environnement veut que ca s'arrete.
	// C'est son geste : pas d'alerte.
	target, err := s.q.GetSiteUpdateTarget(ctx, run.ProjectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("cible du lancement : %w", err)
	}
	if err != nil || !target.Enabled || target.Environment != run.Environment || target.Workflow != run.Workflow || target.Ref != run.Ref {
		if err := s.q.FailSiteUpdateRun(ctx, db.FailSiteUpdateRunParams{ID: run.ID, Message: "Annulé : la cible a changé depuis la demande", At: &now}); err != nil {
			return fmt.Errorf("annulation du lancement : %w", err)
		}
		return nil
	}

	repo, err := s.q.GetProjectRepoURL(ctx, run.ProjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail("Projet introuvable")
	}
	if err != nil {
		return fmt.Errorf("depot du lancement : %w", err)
	}
	if repo != run.RepoUrl || target.RepoUrl != run.RepoUrl {
		return fail("Le dépôt du projet a changé : enregistrez à nouveau la cible pour le confirmer")
	}
	owner, name, ok := githubRepoOf(run.RepoUrl)
	if !ok {
		return fail("Ce projet n’a pas de dépôt GitHub")
	}
	stored, err := s.q.GetSiteUpdateToken(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail("Aucun jeton GitHub enregistré")
	}
	if err != nil {
		return fmt.Errorf("jeton GitHub : %w", err)
	}
	token, err := security.Open(s.sealKey, stored.GithubTokenEnc)
	if err != nil {
		return fail("Le jeton GitHub ne se déchiffre plus : enregistrez-le à nouveau")
	}

	err = s.github.Dispatch(ctx, githubactions.Dispatch{
		Token: token, Owner: owner, Repo: name, Workflow: run.Workflow, Ref: run.Ref,
		Inputs: map[string]string{"environment": run.Environment, "callback_url": s.hookBase + siteUpdateHookURL, "callback_token": callbackToken},
	})
	if err == nil {
		if err := s.q.MarkSiteUpdateDispatched(ctx, db.MarkSiteUpdateDispatchedParams{ID: run.ID, At: &now}); err != nil {
			return fmt.Errorf("lancement demande : %w", err)
		}
		return nil
	}

	var ghErr *githubactions.Error
	isGitHub := errors.As(err, &ghErr)
	if isGitHub && ghErr.Retryable() && int(run.Attempts) < maxDispatchAttempts {
		if err := s.q.RetrySiteUpdateRun(ctx, db.RetrySiteUpdateRunParams{
			ID: run.ID, NextAttemptAt: now.Add(dispatchRetryDelay * time.Duration(run.Attempts)), Message: cutMessage(ghErr.Message),
		}); err != nil {
			return fmt.Errorf("nouvel essai du lancement : %w", err)
		}
		return nil
	}
	if isGitHub && (ghErr.Retryable() || ghErr.Refused()) {
		return fail(ghErr.Message)
	}

	// Le doute : le workflow tourne peut-etre. Pas de nouvel essai, et le jeton
	// reste valable pour que son vrai resultat puisse encore arriver.
	s.log.Warn("lancement de mise a jour", "run", run.ID, "error", err)
	message := "GitHub n’a pas répondu clairement : vérifiez dans le dépôt si le workflow a démarré"
	return s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.MarkSiteUpdateStuck(ctx, db.MarkSiteUpdateStuckParams{ID: run.ID, Status: "failed", Message: message, At: &now}); err != nil {
			return fmt.Errorf("echec du lancement : %w", err)
		}
		return s.alertFailure(ctx, q, run.ProjectID, project, run.Environment, message)
	})
}

func (s *SiteUpdateService) sweepStuck(ctx context.Context, now time.Time) error {
	rows, err := s.q.ListSiteUpdateStuck(ctx, db.ListSiteUpdateStuckParams{DispatchingBefore: now.Add(-dispatchingTimeout), DispatchedBefore: now.Add(-callbackTimeout)})
	if err != nil {
		return fmt.Errorf("lancements sans nouvelles : %w", err)
	}
	var errs []error
	for _, r := range rows {
		status, message := "timed_out", "La CI n’a pas rendu compte dans les temps"
		if r.Status == "dispatching" {
			status, message = "failed", "Interrompu pendant l’appel à GitHub : vérifiez dans le dépôt si le workflow a démarré"
		}
		if err := s.inTx(ctx, func(q *db.Queries) error {
			n, err := q.MarkSiteUpdateStuck(ctx, db.MarkSiteUpdateStuckParams{ID: r.ID, Status: status, Message: message, At: &now})
			if err != nil {
				return fmt.Errorf("lancement sans nouvelles : %w", err)
			}
			if n == 0 {
				return nil
			}
			return s.alertFailure(ctx, q, r.ProjectID, r.ProjectName, r.Environment, message)
		}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *SiteUpdateService) alertFailure(ctx context.Context, q *db.Queries, projectID uuid.UUID, projectName, environment, message string) error {
	where := "préproduction"
	if environment == "prod" {
		where = "production"
	}
	return sendSiteAlert(ctx, q, s.canSend, s.hookBase, siteAlert{
		Kind: "site_update_failed", ProjectID: projectID,
		Subject: fmt.Sprintf("Mise à jour en échec · %s (%s)", projectName, where),
		Message: fmt.Sprintf("La mise à jour de %s en %s a échoué : %s. La planification est arrêtée jusqu’à une relance à la main.", projectName, where, message),
		Payload: map[string]any{"project_name": projectName, "environment": environment, "message": message},
	})
}
