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
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// tickProject : un projet sous formule, avec un site surveille sur tout le mois.
func tickProject(t *testing.T, pool *pgxpool.Pool, planID uuid.UUID, month time.Time, maintenance *usecase.MaintenanceService) portalFixture {
	t.Helper()
	ctx := context.Background()
	client := newPortalClient(t, pool)
	if _, err := maintenance.SetProjectMaintenance(ctx, client.projectID, &planID, month); err != nil {
		t.Fatal(err)
	}
	var monitorID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO uptime_monitors (provider, external_key, name, url, status)
		VALUES ('kuma', $1, 'Vitrine', 'https://vitrine.test', 1) RETURNING id`, "id:"+uuid.NewString()).Scan(&monitorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM uptime_monitors WHERE id = $1`, monitorID) })
	if _, err := pool.Exec(ctx, `INSERT INTO project_uptime_monitors (project_id, monitor_id, environment) VALUES ($1, $2, 'prod')`, client.projectID, monitorID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO uptime_daily (monitor_id, day, checks_total, checks_up, checks_down, ping_sum, ping_count)
		SELECT $1, d::date, 100, 100, 0, 20000, 100
		FROM generate_series($2::date, ($2::date + interval '1 month' - interval '1 day')::date, interval '1 day') d`, monitorID, month); err != nil {
		t.Fatal(err)
	}
	return client
}

func reportOf(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID, month time.Time) (uuid.UUID, string) {
	t.Helper()
	var id uuid.UUID
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT id, status FROM reports WHERE project_id = $1 AND month = $2`, projectID, month).Scan(&id, &status); err != nil {
		t.Fatalf("rapport du projet : %v", err)
	}
	return id, status
}

func TestLePassageDesRapportsNeSeLaissePasBloquer(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	maintenance := usecase.NewMaintenanceService(pool)
	times := usecase.NewTimeEntryService(pool)
	reports := usecase.NewReportService(pool, "https://piilot.test", true, log)

	// L'alerte a besoin de quelqu'un a prevenir.
	createUser(t, pool, "admin")

	plan, err := maintenance.CreatePlan(ctx, usecase.MaintenancePlanInput{
		Name: "Tick " + uuid.NewString()[:8], IncludedMinutes: 60, Sections: []string{"availability", "audience", "interventions"}, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM project_maintenance WHERE plan_id = $1`, plan.ID)
		_ = maintenance.DeletePlan(bg, plan.ID)
		_, _ = pool.Exec(bg, `UPDATE report_settings SET auto_send = false, send_day = 5, wait_days = 3`)
		_, _ = pool.Exec(bg, `DELETE FROM notifications WHERE kind = 'report_attention'`)
	})
	if _, err := pool.Exec(ctx, `UPDATE report_settings SET auto_send = true, send_day = 5, wait_days = 0`); err != nil {
		t.Fatal(err)
	}

	month := time.Date(2031, 2, 1, 0, 0, 0, 0, time.UTC)
	beforeSendDay := time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)
	sendDay := time.Date(2031, 3, 5, 12, 0, 0, 0, time.UTC)

	healthy := tickProject(t, pool, plan.ID, month, maintenance)
	withIncident := tickProject(t, pool, plan.ID, month, maintenance)
	manual := tickProject(t, pool, plan.ID, month, maintenance)
	unwatched := newPortalClient(t, pool)
	if _, err := maintenance.SetProjectMaintenance(ctx, unwatched.projectID, &plan.ID, month); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO uptime_incidents (monitor_id, started_at, ended_at, message, source)
		SELECT monitor_id, '2031-02-12 10:00:00+00', '2031-02-12 10:40:00+00', 'timeout', 'webhook' FROM project_uptime_monitors WHERE project_id = $1`, withIncident.projectID); err != nil {
		t.Fatal(err)
	}

	// Un projet dont la photo d'audience est illisible : il echoue, a chaque
	// passage. Les autres ne doivent rien en voir.
	broken := tickProject(t, pool, plan.ID, month, maintenance)
	var websiteID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO analytics_websites (provider, external_id, name) VALUES ('umami', $1, 'Cassé') RETURNING id`, uuid.NewString()).Scan(&websiteID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM analytics_websites WHERE id = $1`, websiteID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO project_analytics_websites (project_id, website_id, environment) VALUES ($1, $2, 'prod')`, broken.projectID, websiteID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO analytics_snapshots (website_id, month, payload) VALUES ($1, $2, '[]')`, websiteID, month); err != nil {
		t.Fatal(err)
	}

	if _, err := reports.SetRecipients(ctx, manual.projectID, nil, nil, true); err != nil {
		t.Fatal(err)
	}

	// Avant le jour d'envoi : les brouillons, rien ne part. L'echec remonte.
	if err := reports.TickAt(ctx, beforeSendDay); err == nil {
		t.Error("le projet en echec devrait remonter une erreur")
	}
	if _, status := reportOf(t, pool, healthy.projectID, month); status != "draft" {
		t.Errorf("avant le jour d'envoi : %s", status)
	}

	// Une intervention saisie apres la generation entre dans ce qui part.
	team, _ := createUser(t, pool, "team")
	fix := "correctif"
	if _, err := times.Create(ctx, team, usecase.TimeEntryInput{
		ProjectID: healthy.projectID, SpentOn: time.Date(2031, 2, 28, 0, 0, 0, 0, time.UTC), Minutes: 20,
		InterventionKind: &fix, ClientVisible: true, ClientNote: "Saisie en retard",
	}); err != nil {
		t.Fatal(err)
	}

	// Le jour d'envoi.
	if err := reports.TickAt(ctx, sendDay); err == nil {
		t.Error("le projet en echec devrait encore remonter une erreur")
	}
	healthyID, status := reportOf(t, pool, healthy.projectID, month)
	if status != "sent" {
		t.Errorf("le projet sain devrait partir malgre le projet en echec : %s", status)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM email_outbox WHERE report_id = $1`, healthyID)
	})
	sent, err := reports.Get(ctx, healthyID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Data.Interventions == nil || len(sent.Data.Interventions.Items) != 1 {
		t.Errorf("l'intervention saisie en retard manque : %+v", sent.Data.Interventions)
	}
	if sent.EmailsPending != 1 {
		t.Errorf("e-mails en file : %d", sent.EmailsPending)
	}

	incidentID, status := reportOf(t, pool, withIncident.projectID, month)
	if status != "draft" {
		t.Errorf("un incident se relit : %s", status)
	}
	retained, err := reports.Get(ctx, incidentID)
	if err != nil || len(retained.HoldReasons) != 1 || retained.HoldReasons[0] != "incident" {
		t.Errorf("motif de l'incident : %+v, %v", retained.HoldReasons, err)
	}
	if _, status := reportOf(t, pool, manual.projectID, month); status != "draft" {
		t.Errorf("projet en relecture obligatoire : %s", status)
	}
	unwatchedID, status := reportOf(t, pool, unwatched.projectID, month)
	if status != "draft" {
		t.Errorf("projet sans surveillance : %s", status)
	}
	missing, err := reports.Get(ctx, unwatchedID)
	if err != nil || len(missing.HoldReasons) != 1 || missing.HoldReasons[0] != "availability_missing" {
		t.Errorf("motif du projet sans surveillance : %+v, %v", missing.HoldReasons, err)
	}

	// Un admin qui remet un rapport en relecture a le dernier mot : le passage
	// suivant ne l'envoie pas, meme une fois le motif leve.
	if _, err := reports.Validate(ctx, incidentID, team, retained.GeneratedAt.Add(-time.Hour)); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("validation d'une version perimee : %v", err)
	}
	if _, err := reports.Validate(ctx, incidentID, team, retained.GeneratedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Unvalidate(ctx, incidentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM uptime_incidents WHERE started_at = '2031-02-12 10:00:00+00'`); err != nil {
		t.Fatal(err)
	}
	_ = reports.TickAt(ctx, sendDay.Add(time.Hour))
	if _, status := reportOf(t, pool, withIncident.projectID, month); status != "draft" {
		t.Errorf("rapport retenu par un admin : %s", status)
	}

	// Un projet supprime : plus rien ne part.
	if _, err := pool.Exec(ctx, `UPDATE projects SET deleted_at = now() WHERE id = $1`, unwatched.projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET status = 'validated' WHERE id = $1`, unwatchedID); err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Send(ctx, unwatchedID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("envoi pour un projet supprime : %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE projects SET deleted_at = NULL WHERE id = $1`, unwatched.projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET status = 'draft' WHERE id = $1`, unwatchedID); err != nil {
		t.Fatal(err)
	}

	// L'alerte : une seule tant que rien ne s'aggrave.
	count := func() int {
		var alerts int
		if err := pool.QueryRow(ctx, `SELECT count(DISTINCT created_at) FROM notifications WHERE kind = 'report_attention' AND payload->>'month' = '2031-02'`).Scan(&alerts); err != nil {
			t.Fatal(err)
		}
		return alerts
	}
	if count() == 0 {
		t.Fatal("aucune alerte alors que des rapports restent a envoyer")
	}
	// Le test simule 2031 : l'alerte doit dater de ce temps-la, sinon elle
	// paraitrait vieille de plusieurs annees et se repeterait pour cette raison.
	if _, err := pool.Exec(ctx, `UPDATE notifications SET created_at = $1::timestamptz + (created_at - (SELECT max(created_at) FROM notifications WHERE kind = 'report_attention')) WHERE kind = 'report_attention'`, sendDay.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	first := count()
	_ = reports.TickAt(ctx, sendDay.Add(2*time.Hour))
	if got := count(); got != first {
		t.Errorf("l'alerte s'est repetee sans aggravation : %d puis %d", first, got)
	}

	// Un e-mail abandonne : le rapport envoye le dit, et se relance sans
	// changer de contenu.
	if _, err := reports.Resend(ctx, healthyID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("relance sans abandon : %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET undelivered = array_append(undelivered, $2) WHERE id = $1`, healthyID, healthy.email); err != nil {
		t.Fatalf("le declencheur doit laisser passer le suivi de la remise : %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE email_outbox SET status = 'failed' WHERE report_id = $1`, healthyID); err != nil {
		t.Fatal(err)
	}
	resent, err := reports.Resend(ctx, healthyID)
	if err != nil {
		t.Fatal(err)
	}
	if len(resent.Undelivered) != 0 || resent.EmailsPending != 1 || resent.Status != "sent" {
		t.Errorf("relance : %+v", resent)
	}
	if _, err := pool.Exec(ctx, `UPDATE reports SET editorial = 'retouche' WHERE id = $1`, healthyID); err == nil {
		t.Error("la base a laisse modifier un rapport envoye")
	}
}
