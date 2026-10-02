//go:build integration

// Tests d'integration de l'espace team : la page « Mon travail », les
// notifications des tickets et des livrables, et ce que le role team ne voit
// plus.
package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// notificationsOf rend les genres des notifications d'une personne pour un
// objet donne, les plus anciennes d'abord.
func notificationsOf(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, column string, id uuid.UUID) []string {
	t.Helper()

	rows, err := pool.Query(context.Background(),
		`SELECT kind FROM notifications WHERE user_id = $1 AND `+column+` = $2 ORDER BY created_at, id`,
		userID, id)
	if err != nil {
		t.Fatalf("lecture des notifications : %v", err)
	}
	defer rows.Close()

	var kinds []string
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatalf("lecture d'une notification : %v", err)
		}
		kinds = append(kinds, kind)
	}

	return kinds
}

func addMember(t *testing.T, pool *pgxpool.Pool, projectID, userID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO project_members (project_id, user_id) VALUES ($1, $2)`, projectID, userID); err != nil {
		t.Fatalf("ajout a l'equipe : %v", err)
	}
}

func TestUnTicketPrevientQuiLeTraiteEtQuiLaOuvert(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	tickets := usecase.NewTicketService(pool, nil)

	projectID := newProject(t, pool, 0)
	author, _ := createUser(t, pool, "team")
	assignee, _ := createUser(t, pool, "team")

	ticket, err := tickets.Create(ctx, usecase.CreateTicketInput{
		ProjectID: projectID, Subject: "Le formulaire ne part pas", Tracker: "anomalie",
		Priority: "high", AssigneeID: &assignee, CreatedBy: &author,
	})
	if err != nil {
		t.Fatalf("creation du ticket : %v", err)
	}

	if got := notificationsOf(t, pool, assignee, "ticket_id", ticket.ID); len(got) != 1 || got[0] != usecase.NotifyTicketAssigned {
		t.Fatalf("assigne : %v, attendu [ticket_assigned]", got)
	}
	if got := notificationsOf(t, pool, author, "ticket_id", ticket.ID); len(got) != 0 {
		t.Fatalf("l'auteur ne doit rien recevoir de son propre geste : %v", got)
	}

	// La personne qui traite repond et change le statut : l'auteur l'apprend,
	// une seule fois, par le changement de statut qui prime sur la reponse.
	status := "in_progress"
	if _, err := tickets.PostMessage(ctx, ticket.ID, usecase.PostMessageInput{
		Body: "Je regarde", AuthorID: &assignee, NewStatus: &status,
	}); err != nil {
		t.Fatalf("reponse : %v", err)
	}
	if got := notificationsOf(t, pool, author, "ticket_id", ticket.ID); len(got) != 1 || got[0] != usecase.NotifyTicketStatusChanged {
		t.Fatalf("auteur apres reponse : %v, attendu [ticket_status_changed]", got)
	}

	// Une simple reponse.
	if _, err := tickets.PostMessage(ctx, ticket.ID, usecase.PostMessageInput{
		Body: "Corrige en preprod", AuthorID: &assignee,
	}); err != nil {
		t.Fatalf("reponse : %v", err)
	}
	got := notificationsOf(t, pool, author, "ticket_id", ticket.ID)
	if len(got) != 2 || got[1] != usecase.NotifyTicketReplied {
		t.Fatalf("auteur apres seconde reponse : %v", got)
	}
}

func TestUnRetourClientPrevientLEquipeDuProjet(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	deliverables := usecase.NewDeliverableService(pool, nil)

	projectID := newProject(t, pool, 0)
	member, _ := createUser(t, pool, "team")
	outsider, _ := createUser(t, pool, "team")
	decider, _ := createUser(t, pool, "team")
	addMember(t, pool, projectID, member)

	item, err := deliverables.Create(ctx, usecase.CreateDeliverableInput{
		ProjectID: projectID, Title: "Maquettes", URL: "https://example.fr/v1", CreatedBy: &member,
	})
	if err != nil {
		t.Fatalf("creation du livrable : %v", err)
	}

	if _, err := deliverables.Decide(ctx, item.ID, "retours", "Le logo est trop petit", &decider); err != nil {
		t.Fatalf("decision : %v", err)
	}

	if got := notificationsOf(t, pool, member, "deliverable_id", item.ID); len(got) != 1 || got[0] != usecase.NotifyDeliverableFeedback {
		t.Fatalf("membre : %v, attendu [deliverable_feedback]", got)
	}
	if got := notificationsOf(t, pool, outsider, "deliverable_id", item.ID); len(got) != 0 {
		t.Fatalf("hors projet : %v, attendu rien", got)
	}

	// Le livrable a reprendre apparait dans « Mon travail ».
	work, err := usecase.NewMyWorkService(pool).Get(ctx, member)
	if err != nil {
		t.Fatalf("mon travail : %v", err)
	}
	found := false
	for _, d := range work.Deliverables {
		if d.ID == item.ID {
			found = true
			if d.State != "feedback" || d.Feedback != "Le logo est trop petit" {
				t.Fatalf("livrable a reprendre : %+v", d)
			}
		}
	}
	if !found {
		t.Fatal("le livrable renvoye n'apparait pas dans « Mon travail »")
	}

	// Valide, il en sort.
	if _, err := deliverables.Submit(ctx, item.ID, "https://example.fr/v2", &member); err != nil {
		t.Fatalf("nouvelle version : %v", err)
	}
	if _, err := deliverables.Decide(ctx, item.ID, "valide", "", &decider); err != nil {
		t.Fatalf("validation : %v", err)
	}
	work, err = usecase.NewMyWorkService(pool).Get(ctx, member)
	if err != nil {
		t.Fatalf("mon travail : %v", err)
	}
	for _, d := range work.Deliverables {
		if d.ID == item.ID {
			t.Fatal("un livrable valide reste dans « Mon travail »")
		}
	}
}

func TestMonTravailMetLesTachesEnRetardEnTete(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()

	projectID := newProject(t, pool, 0)
	user, _ := createUser(t, pool, "team")
	addMember(t, pool, projectID, user)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	tasks := []struct {
		title  string
		due    *time.Time
		status string
	}{
		{"Sans echeance", nil, "todo"},
		{"Pour la semaine prochaine", ptr(today.AddDate(0, 0, 7)), "progress"},
		{"En retard", ptr(today.AddDate(0, 0, -3)), "todo"},
		{"Terminee", ptr(today.AddDate(0, 0, -5)), "done"},
	}
	for _, task := range tasks {
		var id uuid.UUID
		if err := pool.QueryRow(ctx,
			`INSERT INTO tasks (project_id, title, status, due_on) VALUES ($1, $2, $3, $4) RETURNING id`,
			projectID, task.title, task.status, task.due,
		).Scan(&id); err != nil {
			t.Fatalf("creation de la tache : %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO task_assignees (task_id, user_id) VALUES ($1, $2)`, id, user); err != nil {
			t.Fatalf("affectation : %v", err)
		}
	}

	work, err := usecase.NewMyWorkService(pool).Get(ctx, user)
	if err != nil {
		t.Fatalf("mon travail : %v", err)
	}

	if work.TasksTotal != 3 || work.TasksOverdue != 1 {
		t.Fatalf("comptes : %d ouvertes dont %d en retard, attendu 3 dont 1", work.TasksTotal, work.TasksOverdue)
	}
	order := []string{work.Tasks[0].Title, work.Tasks[1].Title, work.Tasks[2].Title}
	if order[0] != "En retard" || order[1] != "Pour la semaine prochaine" || order[2] != "Sans echeance" {
		t.Fatalf("ordre : %v", order)
	}
	if !work.Tasks[0].Overdue || work.Tasks[1].Overdue {
		t.Fatal("seule la premiere tache est en retard")
	}
	if len(work.Projects) != 1 || work.Projects[0].ID != projectID {
		t.Fatalf("projets : %+v", work.Projects)
	}
}

func TestLeRoleTeamNeVoitPlusLesOutilsDeDirection(t *testing.T) {
	auth, _ := newService(t)
	ctx := context.Background()

	for _, code := range []string{"dashboard.read", "budgets.read", "pipeline.read", "time.read", "roles.read"} {
		team, err := auth.HasPermission(ctx, "team", code)
		if err != nil {
			t.Fatal(err)
		}
		admin, err := auth.HasPermission(ctx, "admin", code)
		if err != nil {
			t.Fatal(err)
		}
		if team || !admin {
			t.Errorf("%s : team=%v admin=%v, attendu false/true", code, team, admin)
		}
	}

	// Ce dont l'equipe a besoin au quotidien reste.
	for _, code := range []string{"projects.read", "time.write", "clients.read", "users.read"} {
		ok, err := auth.HasPermission(ctx, "team", code)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("team a perdu %s", code)
		}
	}
}

func ptr[T any](v T) *T { return &v }
