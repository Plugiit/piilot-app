//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func TestLeRapportMensuelSeFigeEtSEnvoie(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	maintenance := usecase.NewMaintenanceService(pool)
	times := usecase.NewTimeEntryService(pool)
	reports := usecase.NewReportService(pool, "https://piilot.test", true, log)

	client := newPortalClient(t, pool)
	team, _ := createUser(t, pool, "team")
	plan, err := maintenance.CreatePlan(ctx, usecase.MaintenancePlanInput{
		Name: "Rapport " + uuid.NewString()[:8], IncludedMinutes: 60,
		Sections: []string{"availability", "audience", "interventions", "deployments"}, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_maintenance WHERE plan_id = $1`, plan.ID)
		_ = maintenance.DeletePlan(context.Background(), plan.ID)
	})
	september := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := maintenance.SetProjectMaintenance(ctx, client.projectID, &plan.ID, september); err != nil {
		t.Fatal(err)
	}

	// Le mois de septembre : deux interventions dont une interne, une mise en
	// ligne, un site surveille avec un incident.
	fix := "correctif"
	support := "support"
	day := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	if _, err := times.Create(ctx, team, usecase.TimeEntryInput{ProjectID: client.projectID, SpentOn: day, Minutes: 50, InterventionKind: &fix, ClientVisible: true, ClientNote: "Correction du panier"}); err != nil {
		t.Fatal(err)
	}
	if _, err := times.Create(ctx, team, usecase.TimeEntryInput{ProjectID: client.projectID, SpentOn: day, Minutes: 30, InterventionKind: &support}); err != nil {
		t.Fatal(err)
	}
	var monitorID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO uptime_monitors (provider, external_key, name, url, status, cert_days)
		VALUES ('kuma', $1, 'Vitrine', 'https://vitrine.test', 1, 40) RETURNING id`, "id:"+uuid.NewString()).Scan(&monitorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM uptime_monitors WHERE id = $1`, monitorID) })
	for _, q := range []string{
		`INSERT INTO project_uptime_monitors (project_id, monitor_id, environment) VALUES ($1, $2, 'prod')`,
		`INSERT INTO uptime_daily (monitor_id, day, checks_total, checks_up, checks_down, ping_sum, ping_count)
		 SELECT $2, d::date, 100, CASE WHEN d::date = '2026-09-12' THEN 90 ELSE 100 END, CASE WHEN d::date = '2026-09-12' THEN 10 ELSE 0 END, 20000, 100
		 FROM generate_series('2026-09-01'::date, '2026-09-30'::date, interval '1 day') d WHERE $1::uuid IS NOT NULL`,
		`INSERT INTO uptime_incidents (monitor_id, started_at, ended_at, message, source) VALUES ($2, '2026-09-12 10:00:00+02', '2026-09-12 10:40:00+02', 'timeout', 'webhook') RETURNING $1::uuid`,
		`INSERT INTO deployments (project_id, provider, kind, name, environment, url, deployed_at) VALUES ($1, 'github', 'release', 'v2.4.0', 'production', '', '2026-09-20 15:00:00+02') RETURNING $2::uuid`,
	} {
		if _, err := pool.Exec(ctx, q, client.projectID, monitorID); err != nil {
			t.Fatalf("donnees du mois : %v", err)
		}
	}

	// Les brouillons du mois : un seul, meme demande deux fois.
	if n, err := reports.Generate(ctx, september, &client.projectID); err != nil || n != 1 {
		t.Fatalf("generation : %d, %v", n, err)
	}
	if n, _ := reports.Generate(ctx, september, &client.projectID); n != 0 {
		t.Errorf("deuxieme generation : %d", n)
	}
	page, err := reports.List(ctx, september, nil, 1, 25)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	for _, r := range page.Items {
		if r.Project.ID == client.projectID {
			id = r.ID
		}
	}
	report, err := reports.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	d := report.Data
	if d.Availability == nil || len(d.Availability.Sites) != 1 || d.Availability.Sites[0].Uptime == nil || len(d.Availability.Sites[0].Days) != 30 || len(d.Availability.Incidents) != 1 {
		t.Errorf("disponibilite : %+v", d.Availability)
	}
	if d.Availability != nil && *d.Availability.Sites[0].AvgResponseMS != 200 {
		t.Errorf("temps de reponse : %v", *d.Availability.Sites[0].AvgResponseMS)
	}
	// L'interne compte sur le temps inclus, mais n'est pas listee.
	if iv := d.Interventions; iv == nil || len(iv.Items) != 1 || iv.ConsumedMinutes != 80 || iv.VisibleMinutes != 50 || iv.OverrunMinutes != 20 {
		t.Errorf("interventions : %+v", d.Interventions)
	}
	if d.Deployments == nil || len(d.Deployments.Items) != 1 || d.Audience != nil {
		t.Errorf("mises en ligne ou audience : %+v / %+v", d.Deployments, d.Audience)
	}
	if len(report.Recipients) != 1 || report.Recipients[0] != client.email {
		t.Errorf("destinataires prevus : %v", report.Recipients)
	}

	// Pas encore envoye : invisible pour le client.
	if _, err := reports.PortalReport(ctx, client.userID, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("brouillon visible du client : %v", err)
	}

	// Relecture, validation : un rapport valide ne bouge plus sans repasser
	// en brouillon.
	if _, err := reports.SetEditorial(ctx, id, "Un mois calme, une alerte courte le 12."); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Send(ctx, id); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("envoi d'un brouillon : %v", err)
	}
	if _, err := reports.Validate(ctx, id, team); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.SetEditorial(ctx, id, "autre"); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("mot du mois d'un rapport valide : %v", err)
	}

	// Le contact s'exclut, un dirigeant s'ajoute.
	if _, err := reports.SetRecipients(ctx, client.projectID, []uuid.UUID{client.userID}, []string{"dirigeant@client.test", "pas une adresse"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("adresse invalide : %v", err)
	}
	if _, err := reports.SetRecipients(ctx, client.projectID, []uuid.UUID{client.userID}, []string{"Dirigeant@Client.test"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM email_outbox WHERE to_address = 'dirigeant@client.test'`)
	})

	sent, err := reports.Send(ctx, id)
	if err != nil || sent.Status != "sent" || len(sent.Recipients) != 1 || sent.Recipients[0] != "dirigeant@client.test" {
		t.Fatalf("envoi : %+v, %v", sent, err)
	}
	var queued int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM email_outbox WHERE to_address = 'dirigeant@client.test' AND kind = 'monthly_report' AND html_body LIKE '%/client/rapports/' || $1 || '%'`, id.String()).Scan(&queued); err != nil || queued != 1 {
		t.Errorf("e-mail en file : %d, %v", queued, err)
	}

	// Envoye, il ne change plus : ni par le service, ni par la base.
	if _, err := reports.Unvalidate(ctx, id); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("retour en brouillon d'un rapport envoye : %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET editorial = 'retouche' WHERE id = $1`, id); err == nil {
		t.Error("la base a laisse modifier un rapport envoye")
	}

	// Le client le lit dans son archive ; un autre client, non.
	list, err := reports.PortalReports(ctx, client.userID, client.projectID)
	if err != nil || len(list) != 1 || list[0].Month != "2026-09" {
		t.Fatalf("archive : %+v, %v", list, err)
	}
	read, err := reports.PortalReport(ctx, client.userID, id)
	if err != nil || read.Editorial != "Un mois calme, une alerte courte le 12." || read.Data.Interventions.ConsumedMinutes != 80 {
		t.Errorf("rapport du client : %+v, %v", read, err)
	}
	other := newPortalClient(t, pool)
	if _, err := reports.PortalReport(ctx, other.userID, id); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("autre client : %v", err)
	}
	if _, err := reports.PortalReports(ctx, other.userID, client.projectID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("autre client, archive : %v", err)
	}
}
