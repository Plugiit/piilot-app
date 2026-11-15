//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/gitsource"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

const gitSyncSecret = "secret-de-test-suffisamment-long-32"

// fakeGitHub remplace l'API de GitHub : ses pull requests se reglent, et il
// retient le jeton et la periode demandes.
type fakeGitHub struct {
	mu    sync.Mutex
	prs   []gitsource.PullRequest
	err   error
	token string
	since time.Time
	until time.Time
	repo  string
	calls int
}

func (f *fakeGitHub) MergedPullRequests(_ context.Context, repoURL string, since, until time.Time) ([]gitsource.PullRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.repo, f.since, f.until = repoURL, since, until
	return f.prs, f.err
}

func (f *fakeGitHub) set(prs []gitsource.PullRequest, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prs, f.err = prs, err
}

type gitSyncFixture struct {
	svc     *usecase.GitSyncService
	updates *usecase.SiteUpdateService
	fake    *fakeGitHub
	project uuid.UUID
	planID  uuid.UUID
	maint   *usecase.MaintenanceService
}

// gitSyncSetup : un projet sous une formule qui compte les mises a jour, avec
// un depot GitHub. Le jeton de l'agence est retire a la fin.
func gitSyncSetup(t *testing.T, pool *pgxpool.Pool, month time.Time) gitSyncFixture {
	t.Helper()
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f := gitSyncFixture{fake: &fakeGitHub{}, maint: usecase.NewMaintenanceService(pool)}
	f.svc = usecase.NewGitSyncService(pool, []byte(gitSyncSecret), log)
	f.svc.SetProviderFactory(func(token string) gitsource.Provider {
		f.fake.mu.Lock()
		defer f.fake.mu.Unlock()
		f.fake.token = token
		return f.fake
	})
	f.updates = usecase.NewSiteUpdateService(pool, nil, []byte(gitSyncSecret), "https://piilot.test", false, log)

	plan, err := f.maint.CreatePlan(ctx, usecase.MaintenancePlanInput{
		Name: "Git " + uuid.NewString()[:8], IncludedMinutes: 60, Sections: []string{"updates"}, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.planID = plan.ID
	client := newPortalClient(t, pool)
	f.project = client.projectID
	if _, err := f.maint.SetProjectMaintenance(ctx, f.project, &plan.ID, month); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET repo_url = 'https://github.com/agence/site' WHERE id = $1`, f.project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM project_maintenance WHERE plan_id = $1`, plan.ID)
		_ = f.maint.DeletePlan(bg, plan.ID)
		_, _ = pool.Exec(bg, `DELETE FROM site_update_settings`)
		_, _ = pool.Exec(bg, `UPDATE report_settings SET auto_send = false, send_day = 5, wait_days = 3`)
	})
	return f
}

func (f gitSyncFixture) setToken(t *testing.T) {
	t.Helper()
	if _, err := f.updates.SetToken(context.Background(), "github_pat_test"); err != nil {
		t.Fatal(err)
	}
}

func pr(n int64, title, author, branch string, merged time.Time, labels ...string) gitsource.PullRequest {
	opened := merged.Add(-time.Hour)
	return gitsource.PullRequest{Number: n, Title: title, URL: "https://github.com/agence/site/pull/" + title, Branch: branch, Author: author, Labels: labels, OpenedAt: &opened, MergedAt: merged}
}

func TestGitSyncRattrapeUnePRManqueeSansDoublonNiEcrasement(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	march := time.Date(2033, 3, 1, 0, 0, 0, 0, time.UTC)
	f := gitSyncSetup(t, pool, march)
	f.setToken(t)
	now := time.Date(2033, 3, 10, 12, 0, 0, 0, time.UTC)

	// Le webhook a recu la PR 1, avec sa description et ses etiquettes ; il a
	// manque la PR 2 (coupure).
	if _, err := pool.Exec(ctx, `INSERT INTO pull_requests (project_id, provider, number, title, url, branch, author, state, opened_at, merged_at, security, labels)
		VALUES ($1, 'github', 1, 'Titre du webhook', 'https://github.com/agence/site/pull/1', 'renovate/lodash', 'renovate[bot]', 'merged', '2033-03-02 08:00+00', '2033-03-02 09:00+00', true, '{dependencies,security}')`, f.project); err != nil {
		t.Fatal(err)
	}
	f.fake.set([]gitsource.PullRequest{
		pr(1, "Titre de la synchro", "autre", "autre-branche", time.Date(2033, 3, 2, 9, 5, 0, 0, time.UTC), "dependencies"),
		pr(2, "Bump react", "dependabot[bot]", "dependabot/npm_and_yarn/react-19", time.Date(2033, 3, 5, 9, 0, 0, 0, time.UTC), "dependencies"),
		pr(3, "Corrige la faille", "max", "fix/faille", time.Date(2033, 3, 6, 9, 0, 0, 0, time.UTC), "Security"),
	}, nil)

	if err := f.svc.TickAt(ctx, now); err != nil {
		t.Fatal(err)
	}
	if f.fake.token != "github_pat_test" || f.fake.repo != "https://github.com/agence/site" {
		t.Fatalf("le client a recu %q, %q", f.fake.token, f.fake.repo)
	}
	// La periode : du 1er du mois precedent (Paris) jusqu'a demain.
	if want := time.Date(2033, 2, 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour); !f.fake.since.Equal(want) {
		t.Errorf("since = %v, attendu %v", f.fake.since, want)
	}
	if !f.fake.until.After(now) {
		t.Errorf("until = %v", f.fake.until)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pull_requests WHERE project_id = $1`, f.project).Scan(&n); err != nil || n != 3 {
		t.Fatalf("%d pull requests, attendu 3 : %v", n, err)
	}
	var title, author string
	var labels []string
	var security bool
	if err := pool.QueryRow(ctx, `SELECT title, author, labels, security FROM pull_requests WHERE project_id = $1 AND number = 1`, f.project).Scan(&title, &author, &labels, &security); err != nil {
		t.Fatal(err)
	}
	if title != "Titre du webhook" || author != "renovate[bot]" || !slices.Equal(labels, []string{"dependencies", "security"}) || !security {
		t.Errorf("la synchronisation a ecrase le webhook : %q %q %v %v", title, author, labels, security)
	}
	if err := pool.QueryRow(ctx, `SELECT labels, security FROM pull_requests WHERE project_id = $1 AND number = 3`, f.project).Scan(&labels, &security); err != nil || !security || !slices.Equal(labels, []string{"Security"}) {
		t.Errorf("PR 3 : %v %v %v", labels, security, err)
	}

	// Une deuxieme passe, un jour plus tard : toujours trois lignes.
	if err := f.svc.TickAt(ctx, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pull_requests WHERE project_id = $1`, f.project).Scan(&n); err != nil || n != 3 {
		t.Fatalf("apres la deuxieme passe : %d : %v", n, err)
	}
	if f.fake.calls != 2 {
		t.Errorf("%d appels", f.fake.calls)
	}

	view, err := f.svc.Status(ctx, f.project)
	if err != nil || !view.Eligible || !view.TokenConfigured || view.LastSyncedAt == nil || view.LastError != nil || view.Pending {
		t.Fatalf("etat : %+v, %v", view, err)
	}
}

func TestGitSyncAlimenteLeRapport(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	march := time.Date(2033, 3, 1, 0, 0, 0, 0, time.UTC)
	f := gitSyncSetup(t, pool, march)
	f.setToken(t)
	f.fake.set([]gitsource.PullRequest{
		pr(2, "Bump react", "dependabot[bot]", "dependabot/npm_and_yarn/react-19", time.Date(2033, 3, 5, 9, 0, 0, 0, time.UTC), "dependencies"),
		pr(3, "Bump lodash", "max", "chore/lodash", time.Date(2033, 3, 6, 9, 0, 0, 0, time.UTC), "dependencies", "security"),
		pr(4, "Nouvelle page", "max", "feat/page", time.Date(2033, 3, 7, 9, 0, 0, 0, time.UTC), "feature"),
	}, nil)
	if err := f.svc.TickAt(ctx, time.Date(2033, 3, 31, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	reports := usecase.NewReportService(pool, "https://piilot.test", false, log)
	if _, err := reports.Generate(ctx, march, &f.project); err != nil {
		t.Fatal(err)
	}
	u := reportDataOf(t, reports, pool, f.project, march).Data.Updates
	if u == nil || u.DependencyCount != 2 || u.SecurityCount != 1 || u.OtherCount != 1 || len(u.Items) != 2 {
		t.Fatalf("mises a jour du rapport : %+v", u)
	}
}

func TestGitSyncErreursLisiblesSansJetonOuAvecUnJetonSansDroit(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	march := time.Date(2033, 3, 1, 0, 0, 0, 0, time.UTC)
	f := gitSyncSetup(t, pool, march)
	now := time.Date(2033, 3, 10, 12, 0, 0, 0, time.UTC)

	// Sans jeton.
	if err := f.svc.TickAt(ctx, now); err != nil {
		t.Fatal(err)
	}
	view, _ := f.svc.Status(ctx, f.project)
	if view.TokenConfigured || view.LastError == nil || view.LastSyncedAt != nil || view.LastAttemptAt == nil {
		t.Fatalf("sans jeton : %+v", view)
	}
	if f.fake.calls != 0 {
		t.Fatal("GitHub appele sans jeton")
	}

	// Un jeton sans le droit de lire les pull requests.
	f.setToken(t)
	f.fake.set(nil, gitsource.ErrForbidden)
	if _, err := f.svc.RequestSync(ctx, f.project); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TickAt(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	view, _ = f.svc.Status(ctx, f.project)
	if view.LastError == nil || *view.LastError != gitsource.ErrForbidden.Error() || view.Pending {
		t.Fatalf("jeton sans droit : %+v", view)
	}

	// La reprise : l'erreur s'efface, le succes est date.
	f.fake.set(nil, nil)
	if _, err := f.svc.RequestSync(ctx, f.project); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TickAt(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	view, _ = f.svc.Status(ctx, f.project)
	if view.LastError != nil || view.LastSyncedAt == nil {
		t.Fatalf("apres la reprise : %+v", view)
	}

	// Une nouvelle erreur garde la date du dernier succes.
	synced := *view.LastSyncedAt
	f.fake.set(nil, gitsource.ErrUnauthorized)
	if _, err := f.svc.RequestSync(ctx, f.project); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.TickAt(ctx, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	view, _ = f.svc.Status(ctx, f.project)
	if view.LastError == nil || view.LastSyncedAt == nil || !view.LastSyncedAt.Equal(synced) {
		t.Fatalf("apres l'echec : %+v", view)
	}
}

func TestGitSyncQuandSynchroniser(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	march := time.Date(2033, 3, 1, 0, 0, 0, 0, time.UTC)
	f := gitSyncSetup(t, pool, march)
	f.setToken(t)

	// Un projet GitLab, ou sans formule « updates » : jamais lu.
	gitlab := newPortalClient(t, pool)
	if _, err := f.maint.SetProjectMaintenance(ctx, gitlab.projectID, &f.planID, march); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET repo_url = 'https://gitlab.com/agence/site' WHERE id = $1`, gitlab.projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RequestSync(ctx, gitlab.projectID); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("projet GitLab : %v", err)
	}
	if _, err := f.svc.RequestSync(ctx, uuid.New()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("projet inconnu : %v", err)
	}

	calls := func() int { f.fake.mu.Lock(); defer f.fake.mu.Unlock(); return f.fake.calls }
	at := time.Date(2033, 3, 10, 12, 0, 0, 0, time.UTC)
	f.fake.set(nil, nil)
	if err := f.svc.TickAt(ctx, at); err != nil {
		t.Fatal(err)
	}
	if calls() != 1 {
		t.Fatalf("premiere passe : %d appels", calls())
	}
	for _, tc := range []struct {
		name string
		at   time.Time
		want int
	}{
		{"dix minutes plus tard", at.Add(10 * time.Minute), 1},
		{"23 h plus tard", at.Add(23 * time.Hour), 1},
		{"25 h plus tard", at.Add(25 * time.Hour), 2},
	} {
		if err := f.svc.TickAt(ctx, tc.at); err != nil {
			t.Fatal(err)
		}
		if calls() != tc.want {
			t.Fatalf("%s : %d appels, attendu %d", tc.name, calls(), tc.want)
		}
	}
	// La derniere synchronisation date du 11 mars. Le 1er avril a 00 h 05
	// (Paris, heure d'ete : 22 h 05 UTC la veille), moins de 24 h apres le 31 mars
	// a 23 h : la passe du 1er part quand meme.
	if _, err := pool.Exec(ctx, `UPDATE project_git_sync SET last_attempt_at = '2033-03-31 20:00:00+00' WHERE project_id = $1`, f.project); err != nil {
		t.Fatal(err)
	}
	before := calls()
	if err := f.svc.TickAt(ctx, time.Date(2033, 3, 31, 22, 5, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if calls() != before+1 {
		t.Fatalf("passe du 1er : %d appels, attendu %d", calls(), before+1)
	}
}

func TestGitSyncLeRapportAttendLaSynchronisationDuMoisEcoule(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	march := time.Date(2033, 3, 1, 0, 0, 0, 0, time.UTC)
	f := gitSyncSetup(t, pool, march)
	createUser(t, pool, "admin")
	if _, err := pool.Exec(ctx, `UPDATE report_settings SET wait_days = 2`); err != nil {
		t.Fatal(err)
	}
	reports := usecase.NewReportService(pool, "https://piilot.test", false, log)
	firstDay := time.Date(2033, 4, 1, 10, 0, 0, 0, time.UTC)

	// Sans jeton, rien a attendre : le brouillon se cree.
	if err := reports.TickAt(ctx, firstDay); err != nil {
		t.Fatal(err)
	}
	if _, status := reportOf(t, pool, f.project, march); status != "draft" {
		t.Fatalf("sans jeton : %s", status)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM reports WHERE project_id = $1`, f.project); err != nil {
		t.Fatal(err)
	}

	// Avec un jeton, le brouillon attend, et la synchronisation est demandee.
	f.setToken(t)
	if err := reports.TickAt(ctx, firstDay); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE project_id = $1`, f.project).Scan(&n); err != nil || n != 0 {
		t.Fatalf("brouillon cree sans attendre : %d, %v", n, err)
	}
	view, _ := f.svc.Status(ctx, f.project)
	if !view.Pending {
		t.Fatalf("synchronisation non demandee : %+v", view)
	}

	// La tache la fait (une PR de mars, manquee par le webhook) : le brouillon
	// se cree, avec elle.
	f.fake.set([]gitsource.PullRequest{pr(9, "Bump vite", "renovate[bot]", "renovate/vite", time.Date(2033, 3, 20, 9, 0, 0, 0, time.UTC))}, nil)
	if err := f.svc.TickAt(ctx, firstDay.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := reports.TickAt(ctx, firstDay.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if u := reportDataOf(t, reports, pool, f.project, march).Data.Updates; u == nil || u.DependencyCount != 1 {
		t.Fatalf("le rapport ne compte pas la PR rattrapee : %+v", u)
	}

	// Un essai en echec apres la fin du mois ne fait pas attendre non plus.
	if _, err := pool.Exec(ctx, `DELETE FROM reports WHERE project_id = $1`, f.project); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE project_git_sync SET last_attempt_at = '2033-04-01 09:00:00+00', last_error = 'boom' WHERE project_id = $1`, f.project); err != nil {
		t.Fatal(err)
	}
	if err := reports.TickAt(ctx, firstDay); err != nil {
		t.Fatal(err)
	}
	if _, status := reportOf(t, pool, f.project, march); status != "draft" {
		t.Fatalf("apres un essai en echec : %s", status)
	}
}
