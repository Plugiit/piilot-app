package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/gitsource"
	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
	"github.com/plugiit/piilot-app/api/internal/zone"
)

// GitSyncService rattrape les pull requests fusionnees que le webhook a
// manquees, pour que le rapport ne fige pas un mois incomplet.
//
// Il lit GitHub avec le jeton de l'agence (celui des mises a jour de site) et
// ne fait qu'inscrire des pull requests : il ne touche ni aux tickets ni aux
// taches, que le webhook seul fait avancer.
//
// Aucun ecran n'appelle GitHub : « Synchroniser » ne fait que demander, la
// tache de fond fait.
type GitSyncService struct {
	pool    *pgxpool.Pool
	q       *db.Queries
	sealKey []byte
	// newProvider construit le client GitHub avec le jeton ; les tests y
	// branchent un faux.
	newProvider func(token string) gitsource.Provider
	log         *slog.Logger
	now         func() time.Time
}

const (
	// gitSyncLookbackMonths : le mois courant et le precedent, ce qu'un
	// rapport en cours de generation peut encore lire.
	gitSyncLookbackMonths = 1
	// gitSyncEvery : une passe par jour et par projet.
	gitSyncEvery = 24 * time.Hour
)

// NewGitSyncService construit le service. secret est celui qui chiffre le
// jeton GitHub.
func NewGitSyncService(pool *pgxpool.Pool, secret []byte, log *slog.Logger) *GitSyncService {
	return &GitSyncService{
		pool: pool, q: db.New(pool), sealKey: security.DeriveKey(secret, "site-update-github-token"),
		newProvider: func(token string) gitsource.Provider { return &gitsource.GitHub{Token: token} },
		log:         log, now: time.Now,
	}
}

// SetProviderFactory remplace la fabrique du client : les tests y branchent un
// faux fournisseur.
func (s *GitSyncService) SetProviderFactory(f func(token string) gitsource.Provider) {
	s.newProvider = f
}

// GitSyncView est la carte du projet : ou en est la synchronisation.
type GitSyncView struct {
	// Faux : le projet n'a pas de depot GitHub, il n'y a rien a synchroniser.
	Eligible bool `json:"eligible"`
	// Faux : aucun jeton GitHub n'est enregistre.
	TokenConfigured bool       `json:"token_configured"`
	LastSyncedAt    *time.Time `json:"last_synced_at"`
	LastAttemptAt   *time.Time `json:"last_attempt_at"`
	LastError       *string    `json:"last_error"`
	// Une synchronisation demandee n'a pas encore eu lieu.
	Pending bool `json:"pending"`
}

// Status rend l'etat de la synchronisation d'un projet.
func (s *GitSyncService) Status(ctx context.Context, projectID uuid.UUID) (GitSyncView, error) {
	project, err := s.q.GitSyncProject(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && project.Deleted) {
		return GitSyncView{}, domain.ErrNotFound
	}
	if err != nil {
		return GitSyncView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	hasToken, err := s.hasToken(ctx)
	if err != nil {
		return GitSyncView{}, err
	}
	_, _, github := githubRepoOf(project.RepoUrl)
	view := GitSyncView{Eligible: github, TokenConfigured: hasToken}
	row, err := s.q.GetProjectGitSync(ctx, projectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return GitSyncView{}, fmt.Errorf("lecture de la synchronisation : %w", err)
	}
	if err == nil {
		view.LastSyncedAt, view.LastAttemptAt, view.LastError, view.Pending = row.LastSyncedAt, row.LastAttemptAt, row.LastError, row.RequestedAt != nil
	}
	return view, nil
}

func (s *GitSyncService) hasToken(ctx context.Context) (bool, error) {
	_, err := s.q.GetSiteUpdateToken(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("jeton GitHub : %w", err)
	}
	return true, nil
}

// RequestSync demande une synchronisation immediate : le « Synchroniser
// maintenant » de l'ecran. La tache de fond la fait dans la minute.
func (s *GitSyncService) RequestSync(ctx context.Context, projectID uuid.UUID) (GitSyncView, error) {
	project, err := s.q.GitSyncProject(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && project.Deleted) {
		return GitSyncView{}, domain.ErrNotFound
	}
	if err != nil {
		return GitSyncView{}, fmt.Errorf("lecture du projet : %w", err)
	}
	if _, _, ok := githubRepoOf(project.RepoUrl); !ok {
		return GitSyncView{}, domain.ErrValidation.WithDetails(map[string]any{"repo_url": "Ce projet n’a pas de dépôt GitHub"})
	}
	if err := s.q.RequestGitSync(ctx, projectID); err != nil {
		return GitSyncView{}, fmt.Errorf("demande de synchronisation : %w", err)
	}
	return s.Status(ctx, projectID)
}

// Tick est le passage de la tache : chaque projet concerne dont la
// synchronisation est demandee ou due. Une synchronisation due l'est une fois
// par jour, et des le 1er pour le mois qui vient de finir, avant que le
// rapport ne se genere. Un projet en echec n'empeche pas les autres : son
// erreur s'inscrit sur sa fiche.
func (s *GitSyncService) Tick(ctx context.Context) error {
	return s.TickAt(ctx, s.now())
}

// TickAt est le passage de la tache a un instant donne.
func (s *GitSyncService) TickAt(ctx context.Context, now time.Time) error {
	local := now.In(zone.Location())
	firstOfMonth := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, zone.Location())
	from := time.Date(local.Year(), local.Month()-gitSyncLookbackMonths, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	targets, err := s.q.ListGitSyncTargets(ctx, db.ListGitSyncTargetsParams{FromDay: from, ToDay: today})
	if err != nil {
		return fmt.Errorf("projets a synchroniser : %w", err)
	}
	var errs []error
	for _, t := range targets {
		due := t.RequestedAt != nil || t.LastAttemptAt == nil || now.Sub(*t.LastAttemptAt) >= gitSyncEvery || t.LastAttemptAt.Before(firstOfMonth)
		if !due {
			continue
		}
		if err := s.syncProject(ctx, t.ProjectID, t.RepoUrl, now); err != nil {
			s.log.Warn("synchronisation Git", "project", t.ProjectID, "error", err)
		}
	}
	// Les demandes de projets hors formule (bouton) sont servies aussi.
	pending, err := s.q.ListGitSyncRequested(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("demandes en attente : %w", err))
	}
	for _, p := range pending {
		if err := s.syncProject(ctx, p.ProjectID, p.RepoUrl, now); err != nil {
			s.log.Warn("synchronisation Git", "project", p.ProjectID, "error", err)
		}
	}
	return errors.Join(errs...)
}

// syncProject lit GitHub et inscrit ce qui manque. Son erreur est inscrite sur
// la fiche du projet et rendue a l'appelant, qui la journalise.
func (s *GitSyncService) syncProject(ctx context.Context, projectID uuid.UUID, repoURL string, now time.Time) error {
	n, err := s.sync(ctx, projectID, repoURL, now)
	var msg *string
	if err != nil {
		text := err.Error()
		msg = &text
	}
	if rerr := s.q.RecordGitSync(ctx, db.RecordGitSyncParams{ProjectID: projectID, At: now, Error: msg}); rerr != nil {
		return errors.Join(err, fmt.Errorf("enregistrement de la synchronisation : %w", rerr))
	}
	if err == nil {
		s.log.Info("synchronisation Git", "project", projectID, "pull_requests_ajoutees", n)
	}
	return err
}

func (s *GitSyncService) sync(ctx context.Context, projectID uuid.UUID, repoURL string, now time.Time) (int, error) {
	repo := NormalizeRepoURL(repoURL)
	if _, _, ok := githubRepoOf(repo); !ok {
		return 0, gitsource.ErrUnsupportedRepo
	}
	stored, err := s.q.GetSiteUpdateToken(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errors.New("aucun jeton GitHub enregistré : ajoutez-le dans la carte « Mises à jour » du projet")
	}
	if err != nil {
		return 0, fmt.Errorf("jeton GitHub : %w", err)
	}
	token, err := security.Open(s.sealKey, stored.GithubTokenEnc)
	if err != nil {
		return 0, errors.New("le jeton GitHub ne se déchiffre plus : enregistrez-le à nouveau")
	}

	loc := zone.Location()
	local := now.In(loc)
	since := time.Date(local.Year(), local.Month()-gitSyncLookbackMonths, 1, 0, 0, 0, 0, loc)
	prs, err := s.newProvider(token).MergedPullRequests(ctx, repo, since, now.Add(24*time.Hour))
	if err != nil {
		return 0, err
	}

	added := 0
	for _, pr := range prs {
		labels := make([]string, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			if l != "" {
				labels = append(labels, l)
			}
		}
		inserted, err := s.q.UpsertSyncedPullRequest(ctx, db.UpsertSyncedPullRequestParams{
			ProjectID: projectID, Number: pr.Number, Title: pr.Title, Url: pr.URL, Branch: pr.Branch, Author: pr.Author,
			OpenedAt: pr.OpenedAt, MergedAt: &pr.MergedAt, Security: hasSecurityLabel(labels), Labels: labels,
		})
		if err != nil {
			return added, fmt.Errorf("enregistrement de la pull request #%d : %w", pr.Number, err)
		}
		if inserted {
			added++
		}
	}
	return added, nil
}
