//go:build integration

// Tests d'integration du tableau de bord et des budgets.
//
// Ils figent ce que les declencheurs portent et que le code Go ne montre pas :
// l'activite du jour tenue a chaque terminaison de tache, et les heures
// consommees qui font basculer un projet hors budget.
package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// newProject cree un client et un projet jetables, effaces en fin de test.
func newProject(t *testing.T, pool *pgxpool.Pool, hoursSold float64) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var clientID, projectID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO clients (name) VALUES ($1) RETURNING id`, uniqueName("Budget"),
	).Scan(&clientID); err != nil {
		t.Fatalf("creation du client : %v", err)
	}
	dropClient(t, pool, clientID)

	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (client_id, name, status, hours_sold) VALUES ($1, $2, 'production', $3) RETURNING id`,
		clientID, uniqueName("Projet"), hoursSold,
	).Scan(&projectID); err != nil {
		t.Fatalf("creation du projet : %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, projectID); err != nil {
			t.Errorf("nettoyage du projet : %v", err)
		}
	})

	return projectID
}

// tasksDoneToday lit le compteur du jour, tel que la carte d'activite le lit.
func tasksDoneToday(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()

	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT coalesce((SELECT tasks_done FROM daily_activity WHERE day = now()::date), 0)`,
	).Scan(&n); err != nil {
		t.Fatalf("lecture de l'activite : %v", err)
	}

	return n
}

func TestTerminerUneTacheCompteDansLActiviteDuJour(t *testing.T) {
	_, _, pool := newCRM(t)
	ctx := context.Background()
	projectID := newProject(t, pool, 0)

	before := tasksDoneToday(t, pool)

	var taskID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (project_id, title, status) VALUES ($1, 'Tache terminee', 'done') RETURNING id`,
		projectID,
	).Scan(&taskID); err != nil {
		t.Fatalf("creation de la tache : %v", err)
	}
	if got := tasksDoneToday(t, pool); got != before+1 {
		t.Fatalf("apres creation terminee : %d, attendu %d", got, before+1)
	}

	// Rouvrir retire la terminaison : sinon une tache basculee dix fois
	// compterait dix fois.
	if _, err := pool.Exec(ctx, `UPDATE tasks SET status = 'progress' WHERE id = $1`, taskID); err != nil {
		t.Fatalf("reouverture : %v", err)
	}
	if got := tasksDoneToday(t, pool); got != before {
		t.Fatalf("apres reouverture : %d, attendu %d", got, before)
	}

	if _, err := pool.Exec(ctx, `UPDATE tasks SET status = 'done' WHERE id = $1`, taskID); err != nil {
		t.Fatalf("nouvelle terminaison : %v", err)
	}
	if got := tasksDoneToday(t, pool); got != before+1 {
		t.Fatalf("apres nouvelle terminaison : %d, attendu %d", got, before+1)
	}
}

func TestUnProjetQuiDepasseSesHeuresVenduesPasseHorsBudget(t *testing.T) {
	_, _, pool := newCRM(t)
	ctx := context.Background()
	projects := usecase.NewProjectService(pool, nil, 1<<20)
	projectID := newProject(t, pool, 10)

	var userID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, firstname, role) VALUES ($1, 'x', 'Test', 'team') RETURNING id`,
		uniqueName("budget")+"@piilot.test",
	).Scan(&userID); err != nil {
		t.Fatalf("creation du compte : %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("nettoyage du compte : %v", err)
		}
	})

	// 8 h sur 10 vendues : a surveiller.
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (user_id, project_id, spent_on, minutes) VALUES ($1, $2, current_date, 480)`,
		userID, projectID,
	); err != nil {
		t.Fatalf("premiere saisie : %v", err)
	}

	got, err := projects.Get(ctx, projectID, userID)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if got.BudgetState != usecase.BudgetWarning {
		t.Errorf("a 8 h sur 10 : %q, attendu %q", got.BudgetState, usecase.BudgetWarning)
	}

	// 11 h sur 10 : hors budget, et la liste filtree le retrouve.
	if _, err := pool.Exec(ctx,
		`INSERT INTO time_entries (user_id, project_id, spent_on, minutes) VALUES ($1, $2, current_date, 180)`,
		userID, projectID,
	); err != nil {
		t.Fatalf("seconde saisie : %v", err)
	}

	got, err = projects.Get(ctx, projectID, userID)
	if err != nil {
		t.Fatalf("relecture : %v", err)
	}
	if got.BudgetState != usecase.BudgetOver {
		t.Errorf("a 11 h sur 10 : %q, attendu %q", got.BudgetState, usecase.BudgetOver)
	}

	over := usecase.BudgetOver
	page, err := projects.List(ctx, usecase.ProjectFilters{Budget: &over, PageSize: 100, Viewer: userID})
	if err != nil {
		t.Fatalf("liste filtree : %v", err)
	}
	found := false
	for _, item := range page.Items {
		found = found || item.ID == projectID
		if item.BudgetState != usecase.BudgetOver {
			t.Errorf("%s dans le filtre « over » avec l'etat %q", item.Name, item.BudgetState)
		}
	}
	if !found {
		t.Error("le projet hors budget manque a la liste filtree")
	}

	summary, err := projects.Dashboard(ctx)
	if err != nil {
		t.Fatalf("tableau de bord : %v", err)
	}
	if summary.Budget.Over < 1 {
		t.Errorf("budget.over = %d, attendu au moins 1", summary.Budget.Over)
	}
}
