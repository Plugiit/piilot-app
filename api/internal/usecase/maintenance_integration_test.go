//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func newPlan(t *testing.T, pool *pgxpool.Pool, svc *usecase.MaintenanceService, minutes int) usecase.MaintenancePlan {
	t.Helper()
	plan, err := svc.CreatePlan(context.Background(), usecase.MaintenancePlanInput{
		Name: "Sérénité " + uuid.NewString()[:8], IncludedMinutes: minutes,
		Sections: []string{"interventions", "availability"}, Active: true,
	})
	if err != nil {
		t.Fatalf("formule : %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM project_maintenance WHERE plan_id = $1`, plan.ID)
		_ = svc.DeletePlan(context.Background(), plan.ID)
	})
	return plan
}

func consumed(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID, month time.Time) int {
	t.Helper()
	var minutes int
	err := pool.QueryRow(context.Background(), `SELECT coalesce((SELECT minutes FROM maintenance_months WHERE project_id = $1 AND month = $2), 0)`, projectID, month).Scan(&minutes)
	if err != nil {
		t.Fatal(err)
	}
	return minutes
}

func TestLesInterventionsComptentSurLaFormule(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	maintenance := usecase.NewMaintenanceService(pool)
	times := usecase.NewTimeEntryService(pool)

	client := newPortalClient(t, pool)
	userID, _ := createUser(t, pool, "team")
	plan := newPlan(t, pool, maintenance, 60)

	// Les sections se rangent dans l'ordre du rapport ; une inconnue est
	// refusee.
	if plan.Sections[0] != "availability" || plan.Sections[1] != "interventions" {
		t.Errorf("sections : %v", plan.Sections)
	}
	if _, err := maintenance.CreatePlan(ctx, usecase.MaintenancePlanInput{Name: "x", Sections: []string{"seo"}}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("section inconnue : %v", err)
	}

	october := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if _, err := maintenance.SetProjectMaintenance(ctx, client.projectID, &plan.ID, october); err != nil {
		t.Fatalf("formule du projet : %v", err)
	}

	kind := "correctif"
	day := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	// Une intervention visible sans note pour le client est refusee.
	if _, err := times.Create(ctx, userID, usecase.TimeEntryInput{ProjectID: client.projectID, SpentOn: day, Minutes: 45, InterventionKind: &kind, ClientVisible: true}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("visible sans note : %v", err)
	}
	visible, err := times.Create(ctx, userID, usecase.TimeEntryInput{
		ProjectID: client.projectID, SpentOn: day, Minutes: 45, Note: "Corrige le formulaire, cache vide",
		InterventionKind: &kind, ClientVisible: true, ClientNote: "Correction du formulaire de contact",
	})
	if err != nil || visible.Intervention == nil || visible.Intervention.Kind != "correctif" {
		t.Fatalf("intervention : %+v, %v", visible, err)
	}
	support := "support"
	if _, err := times.Create(ctx, userID, usecase.TimeEntryInput{ProjectID: client.projectID, SpentOn: day, Minutes: 30, InterventionKind: &support}); err != nil {
		t.Fatal(err)
	}
	// Un temps ordinaire ne compte pas sur la formule.
	if _, err := times.Create(ctx, userID, usecase.TimeEntryInput{ProjectID: client.projectID, SpentOn: day, Minutes: 120}); err != nil {
		t.Fatal(err)
	}
	if got := consumed(t, pool, client.projectID, october); got != 75 {
		t.Fatalf("consomme d'octobre : %d, attendu 75", got)
	}

	page, err := maintenance.Interventions(ctx, &client.projectID, october, 1, 25)
	if err != nil || page.Total != 2 || len(page.Summaries) != 1 {
		t.Fatalf("liste du mois : %+v, %v", page, err)
	}
	summary := page.Summaries[0]
	if summary.IncludedMinutes == nil || *summary.IncludedMinutes != 60 || summary.ConsumedMinutes != 75 || summary.VisibleMinutes != 45 || summary.OverrunMinutes != 15 {
		t.Errorf("bilan : %+v", summary)
	}

	// Une intervention deplacee en novembre quitte octobre.
	nov := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	if _, err := times.Update(ctx, userID, visible.ID, usecase.TimeEntryInput{
		ProjectID: client.projectID, SpentOn: nov, Minutes: 45, InterventionKind: &kind, ClientVisible: true, ClientNote: "Correction du formulaire de contact",
	}); err != nil {
		t.Fatal(err)
	}
	if got := consumed(t, pool, client.projectID, october); got != 30 {
		t.Errorf("octobre apres deplacement : %d", got)
	}
	if got := consumed(t, pool, client.projectID, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); got != 45 {
		t.Errorf("novembre : %d", got)
	}

	// Le client ne voit que les interventions visibles, avec leur note a lui.
	portal, err := maintenance.PortalInterventions(ctx, client.userID, client.projectID, 1, 20)
	if err != nil || portal.Total != 1 || portal.Items[0].Note != "Correction du formulaire de contact" {
		t.Fatalf("portail : %+v, %v", portal, err)
	}
	other := newPortalClient(t, pool)
	if _, err := maintenance.PortalInterventions(ctx, other.userID, client.projectID, 1, 20); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("autre client : %v", err)
	}

	// Changer de formule en decembre ferme la periode d'octobre la veille.
	bigger := newPlan(t, pool, maintenance, 180)
	view, err := maintenance.SetProjectMaintenance(ctx, client.projectID, &bigger.ID, time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC))
	if err != nil || len(view.History) != 2 || view.Current == nil || view.Current.PlanID != bigger.ID || view.Current.StartsOn != "2026-12-01" {
		t.Fatalf("changement de formule : %+v, %v", view, err)
	}
	if view.History[1].EndsOn == nil || *view.History[1].EndsOn != "2026-11-30" {
		t.Errorf("fin de la premiere periode : %+v", view.History[1])
	}
	// Octobre garde sa formule d'alors.
	page, _ = maintenance.Interventions(ctx, &client.projectID, october, 1, 25)
	if *page.Summaries[0].IncludedMinutes != 60 {
		t.Errorf("octobre a change de formule : %+v", page.Summaries[0])
	}

	// Une formule qui a servi ne se supprime pas.
	if err := maintenance.DeletePlan(ctx, plan.ID); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("suppression d'une formule en usage : %v", err)
	}
}
