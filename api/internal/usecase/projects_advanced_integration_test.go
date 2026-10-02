//go:build integration

// Tests d'integration des projets et du CRM avances : jalons et leurs
// compteurs, modeles de projet, planning, journal des interactions.
package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func clientOf(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), `SELECT client_id FROM projects WHERE id = $1`, projectID).Scan(&id); err != nil {
		t.Fatalf("lecture du client : %v", err)
	}
	return id
}

func interactionKinds(t *testing.T, pool *pgxpool.Pool, clientID uuid.UUID) []string {
	t.Helper()
	feed, err := usecase.NewInteractionService(pool).List(context.Background(), usecase.InteractionFilters{ClientID: &clientID})
	if err != nil {
		t.Fatalf("lecture du journal : %v", err)
	}
	kinds := make([]string, 0, len(feed.Items))
	for _, item := range feed.Items {
		kinds = append(kinds, item.Kind)
	}
	return kinds
}

func TestUnJalonCompteSesLivrablesValides(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	milestones := usecase.NewMilestoneService(pool)
	deliverables := usecase.NewDeliverableService(pool, nil)

	projectID := newProject(t, pool, 0)
	author, _ := createUser(t, pool, "team")

	title := "Maquettes validées"
	due := time.Now().AddDate(0, 0, -2)
	m, err := milestones.Create(ctx, projectID, usecase.MilestoneInput{Title: &title, DueOn: &due}, author)
	if err != nil {
		t.Fatalf("creation du jalon : %v", err)
	}
	if m.State != "late" {
		t.Fatalf("un jalon dont l'echeance est passee est en retard, recu %q", m.State)
	}

	a, err := deliverables.Create(ctx, usecase.CreateDeliverableInput{
		ProjectID: projectID, Title: "Accueil", URL: "https://example.fr/a", CreatedBy: &author, MilestoneID: &m.ID,
	})
	if err != nil {
		t.Fatalf("livrable a : %v", err)
	}
	b, err := deliverables.Create(ctx, usecase.CreateDeliverableInput{
		ProjectID: projectID, Title: "Contact", URL: "https://example.fr/b", CreatedBy: &author,
	})
	if err != nil {
		t.Fatalf("livrable b : %v", err)
	}
	if err := milestones.AttachDeliverable(ctx, b.ID, &m.ID); err != nil {
		t.Fatalf("rattachement : %v", err)
	}
	if _, err := deliverables.Decide(ctx, a.ID, "valide", "", &author); err != nil {
		t.Fatalf("validation : %v", err)
	}

	list, err := milestones.List(ctx, projectID)
	if err != nil {
		t.Fatalf("lecture des jalons : %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].DeliverablesTotal != 2 || list.Items[0].DeliverablesValidated != 1 {
		t.Fatalf("compteurs : %+v", list.Items)
	}
	if len(list.Items[0].Deliverables) != 2 {
		t.Fatalf("livrables sous le jalon : %+v", list.Items[0].Deliverables)
	}

	// Detacher decompte ; un jalon d'un autre projet est refuse.
	if err := milestones.AttachDeliverable(ctx, b.ID, nil); err != nil {
		t.Fatalf("detachement : %v", err)
	}
	other := newProject(t, pool, 0)
	foreign, err := milestones.Create(ctx, other, usecase.MilestoneInput{Title: &title}, author)
	if err != nil {
		t.Fatal(err)
	}
	if err := milestones.AttachDeliverable(ctx, b.ID, &foreign.ID); err == nil {
		t.Fatal("un livrable ne se rattache pas au jalon d'un autre projet")
	}

	done := true
	m2, err := milestones.Update(ctx, m.ID, usecase.MilestoneInput{SetDone: true, Done: done})
	if err != nil {
		t.Fatal(err)
	}
	if m2.State != "done" || m2.DeliverablesTotal != 1 {
		t.Fatalf("apres atteinte : %+v", m2)
	}

	// Le planning le montre a sa date.
	planning, err := milestones.Planning(ctx, author, due.AddDate(0, 0, -1), due.AddDate(0, 0, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range planning.Items {
		if item.ID == m.ID && item.Kind == "milestone" && item.Done {
			found = true
		}
	}
	if !found {
		t.Fatalf("jalon absent du planning : %+v", planning.Items)
	}
	if _, err := milestones.Planning(ctx, author, due, due.AddDate(0, 3, 0), false); err == nil {
		t.Fatal("une periode de trois mois doit etre refusee")
	}
}

func TestUnModeleDonneSesJalonsTachesEtServicesAuProjet(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	author, _ := createUser(t, pool, "admin")
	templates := usecase.NewProjectTemplateService(pool)

	var serviceID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO services (name, color) VALUES ($1, '#ff782b') RETURNING id`, uniqueName("Service")).Scan(&serviceID); err != nil {
		t.Fatalf("service : %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM services WHERE id = $1`, serviceID) })

	five := 5
	tpl, err := templates.Create(ctx, usecase.TemplateInput{
		Name:       uniqueName("Site vitrine"),
		ServiceIDs: []uuid.UUID{serviceID},
		Milestones: []usecase.TemplateMilestone{{Title: "Cadrage", OffsetDays: 0}, {Title: "Mise en ligne", OffsetDays: 30}},
		Tasks: []usecase.TemplateTask{
			{Title: "Atelier de cadrage", Priority: "high", OffsetDays: &five},
			{Title: "Recette"},
		},
	}, author)
	if err != nil {
		t.Fatalf("creation du modele : %v", err)
	}
	t.Cleanup(func() { _ = templates.Delete(context.Background(), tpl.ID) })

	if _, err := templates.Create(ctx, usecase.TemplateInput{Name: tpl.Name}, author); err == nil {
		t.Fatal("deux modeles ne portent pas le meme nom")
	}

	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projects := usecase.NewProjectService(pool, files, 1<<20)
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	project, err := projects.Create(ctx, usecase.CreateProjectInput{
		Name: uniqueName("Projet modele"), ClientName: uniqueName("Client modele"),
		StartsOn: &start, CreatedBy: author, TemplateID: &tpl.ID,
	})
	if err != nil {
		t.Fatalf("creation du projet : %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, project.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, project.ClientID)
	})

	list, err := usecase.NewMilestoneService(pool).List(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 || list.Items[1].Title != "Mise en ligne" || *list.Items[1].DueOn != "2026-12-02" {
		t.Fatalf("jalons copies : %+v", list.Items)
	}
	if project.TasksTotal != 2 {
		t.Fatalf("taches copiees : %d, attendu 2", project.TasksTotal)
	}
	var due *time.Time
	if err := pool.QueryRow(ctx, `SELECT due_on FROM tasks WHERE project_id = $1 AND title = 'Atelier de cadrage'`, project.ID).Scan(&due); err != nil {
		t.Fatal(err)
	}
	if due == nil || due.Format(time.DateOnly) != "2026-11-07" {
		t.Fatalf("echeance de la tache : %v", due)
	}
	var services int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM project_services WHERE project_id = $1`, project.ID).Scan(&services); err != nil {
		t.Fatal(err)
	}
	if services != 1 {
		t.Fatalf("services copies : %d", services)
	}

	// La creation s'inscrit au journal du client.
	if kinds := interactionKinds(t, pool, project.ClientID); len(kinds) != 1 || kinds[0] != usecase.InteractionProjectCreated {
		t.Fatalf("journal apres creation : %v", kinds)
	}
}

func TestLeJournalClientMeleSaisiesEtEvenements(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	interactions := usecase.NewInteractionService(pool)
	author, _ := createUser(t, pool, "team")

	projectID := newProject(t, pool, 0)
	clientID := clientOf(t, pool, projectID)

	if _, err := usecase.NewTicketService(pool, nil).Create(ctx, usecase.CreateTicketInput{
		ProjectID: projectID, Subject: "Erreur 500", Tracker: "anomalie", Priority: "high", CreatedBy: &author,
	}); err != nil {
		t.Fatalf("ticket : %v", err)
	}

	noteID, err := interactions.Create(ctx, clientID, usecase.InteractionInput{
		Kind: usecase.InteractionCall, Body: "Point sur la recette", AuthorID: author,
	})
	if err != nil {
		t.Fatalf("saisie : %v", err)
	}
	if _, err := interactions.Create(ctx, clientID, usecase.InteractionInput{Kind: "ticket_opened", Body: "x", AuthorID: author}); err == nil {
		t.Fatal("un evenement ne se saisit pas a la main")
	}

	kinds := interactionKinds(t, pool, clientID)
	if len(kinds) != 2 || kinds[0] != usecase.InteractionCall || kinds[1] != usecase.InteractionTicketOpened {
		t.Fatalf("journal : %v", kinds)
	}

	feed, err := interactions.List(ctx, usecase.InteractionFilters{ClientID: &clientID, Source: "events"})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("evenements seuls : %d", len(feed.Items))
	}
	if err := interactions.Delete(ctx, feed.Items[0].ID); err == nil {
		t.Fatal("un evenement ne s'efface pas")
	}
	if err := interactions.Delete(ctx, noteID); err != nil {
		t.Fatalf("effacement de la saisie : %v", err)
	}
}
