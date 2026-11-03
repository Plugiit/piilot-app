//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/githubactions"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

type fakeDispatcher struct {
	calls []githubactions.Dispatch
	err   error
}

func (f *fakeDispatcher) Dispatch(_ context.Context, d githubactions.Dispatch) error {
	f.calls = append(f.calls, d)
	return f.err
}

func TestUneMiseAJourPartSurLEnvironnementChoisiEtRendCompte(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	github := &fakeDispatcher{}
	svc := usecase.NewSiteUpdateService(pool, github, []byte("secret-de-test-suffisamment-long-32"), "https://piilot.test", true, log)
	projectID := newProject(t, pool, 0)
	user, _ := createUser(t, pool, "admin")
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM site_update_settings`)
		_, _ = pool.Exec(bg, `DELETE FROM notifications WHERE kind = 'site_update_failed'`)
	})

	// Rien sans choix explicite.
	if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("lancement sans cible : %v", err)
	}
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "", Workflow: "u.yml", Ref: "main", Enabled: true}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("cible sans environnement : %v", err)
	}
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	// Ni depot, ni jeton : refuse clairement.
	if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("lancement sans depot : %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET repo_url = 'https://github.com/agence/site' WHERE id = $1`, projectID); err != nil {
		t.Fatal(err)
	}
	// Le depot a change depuis le choix : rien ne part sans confirmation.
	if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("depot non confirme : %v", err)
	}
	if view, err := svc.View(ctx, projectID); err != nil || !view.RepoChanged {
		t.Errorf("depot change non signale : %+v, %v", view, err)
	}
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("lancement sans jeton : %v", err)
	}
	if _, err := svc.SetToken(ctx, "ghp_fictif"); err != nil {
		t.Fatal(err)
	}
	var sealed []byte
	if err := pool.QueryRow(ctx, `SELECT github_token_enc FROM site_update_settings`).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if string(sealed) == "ghp_fictif" {
		t.Error("le jeton est stocke en clair")
	}

	// L'ecran resté sur l'ancien choix ne lance rien sur le nouveau.
	if _, err := svc.RequestRun(ctx, projectID, user, "prod"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("environnement obsolete : %v", err)
	}

	view, err := svc.RequestRun(ctx, projectID, user, "preprod")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Runs) != 1 || view.Runs[0].Status != "queued" {
		t.Fatalf("lancement : %+v", view.Runs)
	}
	if len(github.calls) != 0 {
		t.Error("GitHub a ete appele pendant la requete")
	}
	if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("deuxieme lancement concurrent : %v", err)
	}

	now := time.Now()
	if err := svc.TickAt(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(github.calls) != 1 {
		t.Fatalf("appels a GitHub : %d", len(github.calls))
	}
	call := github.calls[0]
	if call.Owner != "agence" || call.Repo != "site" || call.Workflow != "piilot-update.yml" || call.Ref != "main" || call.Token != "ghp_fictif" || call.Inputs["environment"] != "preprod" {
		t.Errorf("appel : %+v", call)
	}

	// Pas de second appel au passage suivant.
	if err := svc.TickAt(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(github.calls) != 1 {
		t.Errorf("lance deux fois : %d", len(github.calls))
	}

	// Changer d'environnement ne reecrit pas le lancement en cours.
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "prod", Workflow: "piilot-update.yml", Ref: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	// Le compte rendu : le jeton ne vaut qu'une fois, et un autre ne vaut rien.
	token := call.Inputs["callback_token"]
	if err := svc.RecordResult(ctx, "faux", usecase.SiteUpdateReport{Status: "success"}); !errors.Is(err, usecase.ErrSiteUpdateForbidden) {
		t.Errorf("faux jeton : %v", err)
	}
	if err := svc.RecordResult(ctx, token, usecase.SiteUpdateReport{Status: "bizarre"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("statut inconnu : %v", err)
	}
	if err := svc.RecordResult(ctx, token, usecase.SiteUpdateReport{Status: "success", RunURL: "javascript:alert(1)"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordResult(ctx, token, usecase.SiteUpdateReport{Status: "failed"}); !errors.Is(err, usecase.ErrSiteUpdateForbidden) {
		t.Errorf("jeton rejoue : %v", err)
	}
	view, err = svc.View(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if r := view.Runs[0]; r.Status != "success" || r.Environment != "preprod" || r.RunURL != "" {
		t.Errorf("lancement termine : %+v", r)
	}

	// Silence de la CI : echec et alerte, puis la planification s'arrete.
	everyDay := 1
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "prod", Workflow: "piilot-update.yml", Ref: "main", EveryDays: &everyDay, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	later := now.Add(48 * time.Hour)
	if err := svc.TickAt(ctx, later); err != nil {
		t.Fatal(err)
	}
	if len(github.calls) != 2 || github.calls[1].Inputs["environment"] != "prod" {
		t.Fatalf("lancement planifie : %+v", github.calls)
	}
	silent := later.Add(3 * time.Hour)
	if err := svc.TickAt(ctx, silent); err != nil {
		t.Fatal(err)
	}
	view, err = svc.View(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Runs[0].Status != "timed_out" {
		t.Errorf("silence de la CI : %+v", view.Runs[0])
	}
	var alerts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE kind = 'site_update_failed' AND project_id = $1`, projectID).Scan(&alerts); err != nil || alerts == 0 {
		t.Errorf("alerte : %d, %v", alerts, err)
	}
	// L'alerte part aussi par e-mail : la cloche ne suffit pas si personne ne
	// se connecte. Dans la meme transaction que l'etat, par la file d'envoi.
	var mails int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE kind = 'site_alert' AND html_body LIKE '%' || $1 || '/parametres/maintenance%'`, projectID.String()).Scan(&mails); err != nil || mails != alerts {
		t.Errorf("e-mails d'alerte : %d pour %d notifications, %v", mails, alerts, err)
	}
	if err := svc.TickAt(ctx, silent.Add(72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(github.calls) != 2 {
		t.Errorf("la planification a insiste apres un echec : %d", len(github.calls))
	}

	// Le compte rendu en retard a le dernier mot.
	if err := svc.RecordResult(ctx, github.calls[1].Inputs["callback_token"], usecase.SiteUpdateReport{Status: "success"}); err != nil {
		t.Errorf("compte rendu tardif : %v", err)
	}

	// Enregistrer une frequence ne lance rien dans la minute.
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", EveryDays: &everyDay, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	github.calls = nil
	if err := svc.TickAt(ctx, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(github.calls) != 0 {
		t.Errorf("la frequence a lance une mise a jour : %d", len(github.calls))
	}

	// Desactiver arrete un lancement encore en attente.
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if err := svc.TickAt(ctx, time.Now().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(github.calls) != 0 {
		t.Errorf("une cible desactivee a ete lancee : %d", len(github.calls))
	}
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	// GitHub indisponible : on retente. Un refus : echec. Une coupure : pas de
	// nouvel essai, le workflow tourne peut-etre.
	if _, err := svc.SetTarget(ctx, projectID, usecase.SiteUpdateTargetInput{Environment: "preprod", Workflow: "piilot-update.yml", Ref: "main", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		err   error
		calls int
		want  string
	}{
		{"indisponible", &githubactions.Error{Status: http.StatusServiceUnavailable, Message: "Service Unavailable"}, 1, "queued"},
		{"reponse douteuse", &githubactions.Error{Status: http.StatusBadGateway, Message: "Bad Gateway"}, 1, "failed"},
		{"refus", &githubactions.Error{Status: http.StatusNotFound, Message: "Not Found"}, 1, "failed"},
		{"coupure", errors.New("timeout"), 1, "failed"},
	}
	at := time.Now().Add(100 * time.Hour)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _ = pool.Exec(ctx, `UPDATE site_update_runs SET status = 'failed', finished_at = now(), callback_token_hash = NULL WHERE project_id = $1 AND status IN ('queued', 'dispatching', 'dispatched')`, projectID)
			if _, err := svc.RequestRun(ctx, projectID, user, "preprod"); err != nil {
				t.Fatal(err)
			}
			github.calls, github.err = nil, c.err
			at = at.Add(time.Hour)
			_ = svc.TickAt(ctx, at)
			if len(github.calls) != c.calls {
				t.Errorf("appels : %d", len(github.calls))
			}
			view, err := svc.View(ctx, projectID)
			if err != nil {
				t.Fatal(err)
			}
			if view.Runs[0].Status != c.want {
				t.Errorf("etat : %+v", view.Runs[0])
			}
		})
	}
}

func TestLesSauvegardesSeSignalentEtLeSilenceAlerte(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	expiry := &fakeExpiry{}
	svc := usecase.NewSiteSourcesService(pool, expiry, "https://piilot.test", true, log)
	projectID := newProject(t, pool, 0)
	createUser(t, pool, "admin")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM notifications WHERE kind IN ('site_backup_stale', 'domain_expiring', 'domain_check_failed') AND project_id = $1`, projectID)
	})

	token, err := svc.RotateBackupToken(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordBackup(ctx, "faux", usecase.SiteBackupReport{Status: "success"}); !errors.Is(err, usecase.ErrSiteBackupForbidden) {
		t.Errorf("faux jeton : %v", err)
	}
	if err := svc.RecordBackup(ctx, token.Token, usecase.SiteBackupReport{Status: "bizarre"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("statut inconnu : %v", err)
	}
	backupSize := int64(1024)
	if err := svc.RecordBackup(ctx, token.Token, usecase.SiteBackupReport{Status: "success", SizeBytes: &backupSize}); err != nil {
		t.Fatal(err)
	}
	view, err := svc.View(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Backup == nil || !view.Backup.Healthy || len(view.Backup.Runs) != 1 {
		t.Errorf("sauvegarde saine : %+v", view.Backup)
	}

	// Un nouveau jeton remplace l'ancien.
	if _, err := svc.RotateBackupToken(ctx, projectID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RecordBackup(ctx, token.Token, usecase.SiteBackupReport{Status: "success"}); !errors.Is(err, usecase.ErrSiteBackupForbidden) {
		t.Errorf("ancien jeton : %v", err)
	}

	// Le silence : on recule la derniere reussite, le passage alerte, une fois.
	if _, err := pool.Exec(ctx, `UPDATE site_backup_sources SET last_success_at = now() - interval '3 days' WHERE project_id = $1`, projectID); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(DISTINCT created_at) FROM notifications WHERE kind = 'site_backup_stale' AND project_id = $1`, projectID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Errorf("alerte du silence : %d", got)
	}
	var mails int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE kind = 'site_alert' AND subject LIKE 'Sauvegardes muettes%' AND html_body LIKE '%' || $1 || '%'`, projectID.String()).Scan(&mails); err != nil || mails == 0 {
		t.Errorf("e-mail du silence : %d, %v", mails, err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Errorf("alerte repetee : %d", got)
	}

	// Un domaine : la date arrive par la tache, jamais par la requete.
	expiry.at = time.Now().Add(20 * 24 * time.Hour)
	view, err = svc.AddDomain(ctx, projectID, "https://www.Exemple-"+uuid.NewString()[:8]+".fr/page")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Domains) != 1 || view.Domains[0].ExpiresAt != nil {
		t.Fatalf("domaine ajoute : %+v", view.Domains)
	}
	if expiry.calls != 0 {
		t.Error("RDAP a ete appele pendant la requete")
	}
	if _, err := svc.AddDomain(ctx, projectID, view.Domains[0].Domain); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("domaine en double : %v", err)
	}
	if err := svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	view, err = svc.View(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Domains[0].DaysLeft == nil || *view.Domains[0].DaysLeft > 20 {
		t.Errorf("domaine releve : %+v", view.Domains[0])
	}
	var alerts int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT created_at) FROM notifications WHERE kind = 'domain_expiring' AND project_id = $1`, projectID).Scan(&alerts); err != nil || alerts != 1 {
		t.Errorf("alerte du domaine : %d, %v", alerts, err)
	}
}

type fakeExpiry struct {
	at    time.Time
	calls int
}

func (f *fakeExpiry) Expiry(context.Context, string) (time.Time, error) {
	f.calls++
	return f.at, nil
}
