//go:build integration

package usecase_test

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/coolify"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// Mois fictifs, loin devant : les rapports des autres tests ne les voient pas.
// Septembre est en heure d'ete (UTC+2), la bascule d'octobre est le 31.
var (
	coolifySeptember = time.Date(2032, 9, 1, 0, 0, 0, 0, time.UTC)
	coolifyOctober   = time.Date(2032, 10, 1, 0, 0, 0, 0, time.UTC)
)

func utc(y int, m time.Month, d, h, min int) time.Time {
	return time.Date(y, m, d, h, min, 0, 0, time.UTC)
}

// linkedProject : un projet lie a l'application a1 en production, le module
// actif et la liste des applications relevee.
func linkedProject(t *testing.T, pool *pgxpool.Pool, fake *fakeCoolify, svc *usecase.CoolifyService) portalFixture {
	t.Helper()
	ctx := context.Background()
	fake.set([]coolify.Application{{UUID: "a1", Name: "Vitrine", FQDN: "https://vitrine.test,https://www.vitrine.test"}}, nil)
	token := "1|t"
	if _, err := svc.UpdateSettings(ctx, usecase.CoolifySettingsInput{Enabled: true, BaseURL: "https://coolify.test", APIToken: &token}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Poll(ctx); err != nil {
		t.Fatalf("releve des applications : %v", err)
	}
	client := newPortalClient(t, pool)
	view, err := svc.SetProjectApplications(ctx, client.projectID, []usecase.ProjectCoolifyInput{{ApplicationUUID: "a1", Environment: "prod"}})
	if err != nil || len(view.Applications) != 1 || view.Applications[0].Name != "Vitrine" || !view.Applications[0].Present {
		t.Fatalf("liaison : %+v, %v", view, err)
	}
	return client
}

func countDeployments(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID, provider string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM deployments WHERE project_id = $1 AND provider = $2`, projectID, provider).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCoolifyLaReleveDesDeploiementsNEcritQueLesSuccesEtResteIdempotente(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	client := linkedProject(t, pool, fake, svc)

	fake.mu.Lock()
	fake.deploys = map[string][]coolify.Deployment{"a1": {
		{UUID: "d1", Status: "finished", CommitSHA: "0123456789abcdef", CommitMessage: "fix: corrige le panier\n\ndétails", CreatedAt: utc(2032, 9, 10, 8, 0), UpdatedAt: utc(2032, 9, 10, 8, 5)},
		{UUID: "d2", Status: "FINISHED", CommitSHA: "0123456789abcdef", CreatedAt: utc(2032, 9, 12, 8, 0), UpdatedAt: utc(2032, 9, 12, 8, 5)},
		{UUID: "d3", Status: "failed", CommitMessage: "echec", CreatedAt: utc(2032, 9, 13, 8, 0)},
		{UUID: "d4", Status: "in_progress", CreatedAt: utc(2032, 9, 14, 8, 0)},
		{UUID: "d5", Status: "mystere", CreatedAt: utc(2032, 9, 15, 8, 0)},
		{Status: "finished", CommitMessage: "sans uuid", CreatedAt: utc(2032, 9, 16, 8, 0), UpdatedAt: utc(2032, 9, 16, 8, 1)},
		{UUID: "d7", Status: "finished", CommitMessage: "sans date"},
	}}
	fake.mu.Unlock()

	for i := 0; i < 2; i++ {
		if err := svc.Poll(ctx); err != nil {
			t.Fatalf("releve %d : %v", i+1, err)
		}
	}
	// d1, d2 et celui sans uuid : trois lignes, pas six. Ni echec, ni statut
	// inconnu, ni deploiement sans date.
	if n := countDeployments(t, pool, client.projectID, "coolify"); n != 3 {
		t.Fatalf("%d deploiements, attendu 3", n)
	}

	view, err := svc.ProjectCoolify(ctx, client.projectID)
	if err != nil || len(view.Deployments) != 3 {
		t.Fatalf("carte du projet : %+v, %v", view, err)
	}
	byName := map[string]usecase.ProjectCoolifyDeployment{}
	for _, d := range view.Deployments {
		byName[d.Name] = d
	}
	d1 := byName["fix: corrige le panier"]
	if d1.Environment != "production" || d1.URL != "https://vitrine.test" || !d1.DeployedAt.Equal(utc(2032, 9, 10, 8, 5)) {
		t.Errorf("d1 = %+v", d1)
	}
	if _, ok := byName["01234567"]; !ok {
		t.Errorf("le nom d'un deploiement sans message est le commit court : %+v", byName)
	}
	if _, ok := byName["sans uuid"]; !ok {
		t.Errorf("deploiement sans uuid : %+v", byName)
	}

	// Un module inactif ne releve que les applications.
	if _, err := pool.Exec(ctx, `UPDATE coolify_settings SET enabled = false`); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.deploys["a1"] = append(fake.deploys["a1"], coolify.Deployment{UUID: "d8", Status: "finished", CreatedAt: utc(2032, 9, 20, 8, 0)})
	fake.mu.Unlock()
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countDeployments(t, pool, client.projectID, "coolify"); n != 3 {
		t.Fatalf("module inactif : %d deploiements", n)
	}
}

func TestCoolifyUneApplicationEnErreurNArretePasLesAutres(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	client := linkedProject(t, pool, fake, svc)

	fake.mu.Lock()
	fake.depErr = coolify.ErrForbidden
	fake.mu.Unlock()
	if err := svc.Poll(ctx); err == nil {
		t.Fatal("une releve des deploiements ratee n'a pas ete signalee")
	}
	view, _ := svc.Settings(ctx)
	if view.Status.State != "error" || view.Status.LastPollError == nil {
		t.Fatalf("etat : %+v", view.Status)
	}
	if n := countDeployments(t, pool, client.projectID, "coolify"); n != 0 {
		t.Fatalf("%d deploiements ecrits malgre l'erreur", n)
	}
}

func TestCoolifyLiaisonDUnProjet(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	client := linkedProject(t, pool, fake, svc)

	for name, in := range map[string][]usecase.ProjectCoolifyInput{
		"inconnue":      {{ApplicationUUID: "zzz", Environment: "prod"}},
		"environnement": {{ApplicationUUID: "a1", Environment: "dev"}},
		"doublon":       {{ApplicationUUID: "a1", Environment: "prod"}, {ApplicationUUID: "a1", Environment: "preprod"}},
	} {
		if _, err := svc.SetProjectApplications(ctx, client.projectID, in); err == nil {
			t.Errorf("%s : acceptee", name)
		}
	}
	if _, err := svc.SetProjectApplications(ctx, uuid.New(), nil); err == nil {
		t.Error("projet inconnu accepte")
	}
	// Les refus n'ont rien change.
	if view, _ := svc.ProjectCoolify(ctx, client.projectID); len(view.Applications) != 1 {
		t.Fatalf("liaison perdue : %+v", view)
	}
	view, err := svc.SetProjectApplications(ctx, client.projectID, nil)
	if err != nil || len(view.Applications) != 0 {
		t.Fatalf("detachement : %+v, %v", view, err)
	}
}

// coolifyReportPlan : une formule qui ne compte que les mises en ligne.
func coolifyReportPlan(t *testing.T, pool *pgxpool.Pool, maintenance *usecase.MaintenanceService) uuid.UUID {
	t.Helper()
	plan, err := maintenance.CreatePlan(context.Background(), usecase.MaintenancePlanInput{
		Name: "Coolify " + uuid.NewString()[:8], IncludedMinutes: 60, Sections: []string{"deployments"}, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM project_maintenance WHERE plan_id = $1`, plan.ID)
		_ = maintenance.DeletePlan(bg, plan.ID)
		_, _ = pool.Exec(bg, `UPDATE report_settings SET auto_send = false, send_day = 5, wait_days = 3`)
	})
	return plan.ID
}

func reportDataOf(t *testing.T, reports *usecase.ReportService, pool *pgxpool.Pool, projectID uuid.UUID, month time.Time) usecase.ReportView {
	t.Helper()
	id, _ := reportOf(t, pool, projectID, month)
	report, err := reports.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestCoolifyDansLeRapportBornesDuMoisEtPrioriteSurLeWebhook(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	maintenance := usecase.NewMaintenanceService(pool)
	reports := usecase.NewReportService(pool, "https://piilot.test", true, log)
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	planID := coolifyReportPlan(t, pool, maintenance)

	client := linkedProject(t, pool, fake, svc)
	for _, month := range []time.Time{coolifySeptember, coolifyOctober} {
		if _, err := maintenance.SetProjectMaintenance(ctx, client.projectID, &planID, month); err != nil {
			t.Fatal(err)
		}
	}
	// Un webhook GitHub sur chaque mois : tant que Coolify n'a rien releve, il
	// fait foi.
	for _, q := range []string{
		`INSERT INTO deployments (project_id, provider, kind, name, environment, deployed_at) VALUES ($1, 'github', 'release', 'v1-webhook', 'production', '2032-09-20 12:00:00+00')`,
		`INSERT INTO deployments (project_id, provider, kind, name, environment, deployed_at) VALUES ($1, 'github', 'release', 'v2-webhook', 'production', '2032-10-20 12:00:00+00')`,
	} {
		if _, err := pool.Exec(ctx, q, client.projectID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := reports.Generate(ctx, coolifySeptember, &client.projectID); err != nil {
		t.Fatal(err)
	}
	if d := reportDataOf(t, reports, pool, client.projectID, coolifySeptember).Data.Deployments; d == nil || d.Source != "webhook" || len(d.Items) != 1 || d.Items[0].Name != "v1-webhook" {
		t.Fatalf("sans Coolify : %+v", d)
	}

	// Coolify releve : le 30 septembre a 23 h 30 Paris (21 h 30 UTC) est en
	// septembre, a 00 h 30 Paris (22 h 30 UTC) il est en octobre.
	fake.mu.Lock()
	fake.deploys = map[string][]coolify.Deployment{"a1": {
		{UUID: "sep-last", Status: "finished", CommitMessage: "dernier de septembre", CreatedAt: utc(2032, 9, 30, 21, 20), UpdatedAt: utc(2032, 9, 30, 21, 30)},
		{UUID: "oct-first", Status: "finished", CommitMessage: "premier d'octobre", CreatedAt: utc(2032, 9, 30, 22, 20), UpdatedAt: utc(2032, 9, 30, 22, 30)},
		{UUID: "oct-mid", Status: "finished", CommitMessage: "milieu d'octobre", CreatedAt: utc(2032, 10, 15, 10, 0), UpdatedAt: utc(2032, 10, 15, 10, 5)},
		{UUID: "oct-last", Status: "finished", CommitMessage: "dernier d'octobre", CreatedAt: utc(2032, 10, 31, 22, 50), UpdatedAt: utc(2032, 10, 31, 22, 59)},
		{UUID: "nov-first", Status: "finished", CommitMessage: "premier de novembre", CreatedAt: utc(2032, 10, 31, 23, 20), UpdatedAt: utc(2032, 10, 31, 23, 30)},
	}}
	fake.mu.Unlock()
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := reports.Generate(ctx, coolifyOctober, &client.projectID); err != nil {
		t.Fatal(err)
	}
	october := reportDataOf(t, reports, pool, client.projectID, coolifyOctober).Data.Deployments
	var names []string
	if october != nil {
		if october.Source != "coolify" {
			t.Errorf("source d'octobre : %q", october.Source)
		}
		for _, i := range october.Items {
			names = append(names, i.Name)
		}
	}
	if want := []string{"premier d'octobre", "milieu d'octobre", "dernier d'octobre"}; !slices.Equal(names, want) {
		t.Errorf("octobre : %v, attendu %v", names, want)
	}

	// Septembre figure deja au rapport : le brouillon se remet a jour, et
	// Coolify l'emporte sur le webhook.
	september := reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if _, err := reports.Regenerate(ctx, september.ID); err != nil {
		t.Fatalf("mise a jour du brouillon : %v", err)
	}
	d := reportDataOf(t, reports, pool, client.projectID, coolifySeptember).Data.Deployments
	if d == nil || d.Source != "coolify" || len(d.Items) != 1 || d.Items[0].Name != "dernier de septembre" {
		t.Errorf("septembre apres la releve : %+v", d)
	}
}

func TestCoolifyUnRapportEnvoyeNeChangePasApresUneNouvelleReleve(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	maintenance := usecase.NewMaintenanceService(pool)
	reports := usecase.NewReportService(pool, "https://piilot.test", true, log)
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	planID := coolifyReportPlan(t, pool, maintenance)
	team, _ := createUser(t, pool, "team")

	client := linkedProject(t, pool, fake, svc)
	if _, err := maintenance.SetProjectMaintenance(ctx, client.projectID, &planID, coolifySeptember); err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.deploys = map[string][]coolify.Deployment{"a1": {{UUID: "d1", Status: "finished", CommitMessage: "un", CreatedAt: utc(2032, 9, 10, 8, 0), UpdatedAt: utc(2032, 9, 10, 8, 5)}}}
	fake.mu.Unlock()
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Generate(ctx, coolifySeptember, &client.projectID); err != nil {
		t.Fatal(err)
	}
	report := reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if _, err := reports.Validate(ctx, report.ID, team, report.GeneratedAt); err != nil {
		t.Fatal(err)
	}

	// Un deploiement de septembre apparait apres coup : la releve l'ecrit,
	// mais le rapport valide, lui, ne bouge pas.
	fake.mu.Lock()
	fake.deploys["a1"] = append(fake.deploys["a1"], coolify.Deployment{UUID: "d2", Status: "finished", CommitMessage: "deux", CreatedAt: utc(2032, 9, 11, 8, 0), UpdatedAt: utc(2032, 9, 11, 8, 5)})
	fake.mu.Unlock()
	if err := svc.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countDeployments(t, pool, client.projectID, "coolify"); n != 2 {
		t.Fatalf("%d deploiements", n)
	}
	after := reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if d := after.Data.Deployments; d == nil || len(d.Items) != 1 || d.Items[0].Name != "un" {
		t.Errorf("le rapport valide a change : %+v", d)
	}
}

func TestCoolifyLeMotifDeRetenueEtLAttenteDeLaReleve(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	maintenance := usecase.NewMaintenanceService(pool)
	reports := usecase.NewReportService(pool, "https://piilot.test", true, log)
	svc, fake := newCoolify(t, pool, config.CoolifyEnv{})
	planID := coolifyReportPlan(t, pool, maintenance)
	createUser(t, pool, "admin")
	if _, err := pool.Exec(ctx, `UPDATE report_settings SET wait_days = 2`); err != nil {
		t.Fatal(err)
	}

	client := linkedProject(t, pool, fake, svc)
	other := newPortalClient(t, pool) // sans lien Coolify : jamais retenu pour lui
	for _, p := range []uuid.UUID{client.projectID, other.projectID} {
		if _, err := maintenance.SetProjectMaintenance(ctx, p, &planID, coolifySeptember); err != nil {
			t.Fatal(err)
		}
	}
	// La derniere releve date d'avant la fin de septembre.
	setPoll := func(at string, errMsg *string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE coolify_settings SET last_poll_at = $1::timestamptz, last_poll_error = $2, poll_requested_at = NULL`, at, errMsg); err != nil {
			t.Fatal(err)
		}
	}
	setPoll("2032-09-15 12:00:00+00", nil)

	// Le 1er octobre, dans le delai d'attente : le brouillon du projet suivi
	// attend la releve, et la demande ; l'autre part sans attendre.
	firstDay := time.Date(2032, 10, 1, 10, 0, 0, 0, time.UTC)
	if err := reports.TickAt(ctx, firstDay); err != nil {
		t.Fatal(err)
	}
	var requested bool
	if err := pool.QueryRow(ctx, `SELECT poll_requested_at IS NOT NULL FROM coolify_settings`).Scan(&requested); err != nil || !requested {
		t.Fatalf("releve non demandee : %v, %v", requested, err)
	}
	if _, status := reportOf(t, pool, other.projectID, coolifySeptember); status != "draft" {
		t.Errorf("projet sans Coolify : %s", status)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE project_id = $1`, client.projectID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("le brouillon du projet suivi a ete cree sans attendre : %d, %v", n, err)
	}

	// La releve passe apres la fin du mois : le brouillon se cree, sans motif.
	setPoll("2032-10-01 10:05:00+00", nil)
	if err := reports.TickAt(ctx, firstDay.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	r := reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if r.Data.DeploymentsStale || slices.Contains(r.HoldReasons, "deployments_stale") {
		t.Errorf("releve a jour : %+v", r.HoldReasons)
	}

	// Une releve en erreur retient, et ne fait pas attendre.
	failed := "Coolify répond 502"
	setPoll("2032-10-01 11:00:00+00", &failed)
	if _, err := reports.Regenerate(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	r = reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if !r.Data.DeploymentsStale || !slices.Contains(r.HoldReasons, "deployments_stale") {
		t.Errorf("releve en erreur : %+v", r.HoldReasons)
	}
	// Une releve plus ancienne que le mois retient aussi.
	setPoll("2032-09-30 12:00:00+00", nil)
	if _, err := reports.Regenerate(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	r = reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if !slices.Contains(r.HoldReasons, "deployments_stale") {
		t.Errorf("releve trop ancienne : %+v", r.HoldReasons)
	}
	// Module inactif : plus rien ne retient.
	if _, err := pool.Exec(ctx, `UPDATE coolify_settings SET enabled = false`); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Regenerate(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	r = reportDataOf(t, reports, pool, client.projectID, coolifySeptember)
	if slices.Contains(r.HoldReasons, "deployments_stale") {
		t.Errorf("module inactif : %+v", r.HoldReasons)
	}
	// Le projet sans lien n'a jamais ete retenu pour cette raison.
	if o := reportDataOf(t, reports, pool, other.projectID, coolifySeptember); slices.Contains(o.HoldReasons, "deployments_stale") {
		t.Errorf("projet sans lien : %+v", o.HoldReasons)
	}
}
