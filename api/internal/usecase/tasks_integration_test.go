//go:build integration

// Tests d'integration de l'ecran « Taches ».
//
// Ils figent le plafond par colonne du kanban, qui a remplace un plafond global
// : trie par statut, celui-ci donnait toutes les places aux taches terminees et
// vidait les colonnes actives des que l'agence depassait trois cents taches.
package usecase_test

import (
	"context"
	"testing"

	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func TestLeKanbanGlobalPlafonneChaqueColonneSansVoirLesAutres(t *testing.T) {
	_, _, pool := newCRM(t)
	ctx := context.Background()
	tasks := usecase.NewTaskService(pool, nil, 1<<20, nil)
	projectID := newProject(t, pool, 0)

	// 55 terminees, plus que le plafond d'une colonne, et 3 a faire.
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks (project_id, title, status)
		SELECT $1, 'Terminee ' || n, 'done' FROM generate_series(1, 55) AS n`, projectID,
	); err != nil {
		t.Fatalf("creation des taches terminees : %v", err)
	}
	// Terminees a une heure d'ecart : la plus recente est « Terminee 1 ».
	if _, err := pool.Exec(ctx, `
		UPDATE tasks SET completed_at = now() - (split_part(title, ' ', 2)::int * interval '1 hour')
		WHERE project_id = $1 AND status = 'done'`, projectID,
	); err != nil {
		t.Fatalf("datation des terminaisons : %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO tasks (project_id, title, status)
		SELECT $1, 'A faire ' || n, 'todo' FROM generate_series(1, 3) AS n`, projectID,
	); err != nil {
		t.Fatalf("creation des taches a faire : %v", err)
	}

	board, err := tasks.GlobalBoard(ctx, usecase.TaskFilters{ProjectID: &projectID})
	if err != nil {
		t.Fatalf("kanban : %v", err)
	}

	shown := map[string]int{}
	for _, item := range board.Items {
		shown[item.Status]++
	}
	if shown["todo"] != 3 {
		t.Errorf("colonne a faire : %d cartes, attendu 3 — les terminees ne doivent pas la vider", shown["todo"])
	}
	if shown["done"] != board.ColumnLimit {
		t.Errorf("colonne terminee : %d cartes, attendu le plafond %d", shown["done"], board.ColumnLimit)
	}
	if board.Columns.Done != 55 || board.Columns.Todo != 3 {
		t.Errorf("totaux = %+v, attendu 55 terminees et 3 a faire", board.Columns)
	}

	// Dans l'ordre du flux, et la terminee la plus recente en tete de sa colonne.
	if board.Items[0].Status != "todo" {
		t.Errorf("premiere carte en %q, attendu todo", board.Items[0].Status)
	}
	for _, item := range board.Items {
		if item.Status == "done" {
			if item.Title != "Terminee 1" {
				t.Errorf("premiere terminee : %q, attendu la plus recente", item.Title)
			}
			break
		}
	}

	page, err := tasks.List(ctx, usecase.TaskFilters{ProjectID: &projectID, Page: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("liste : %v", err)
	}
	if page.Total != 58 || len(page.Items) != 10 || page.Page != 2 {
		t.Errorf("page 2 : total %d, %d lignes, page %d ; attendu 58, 10, 2", page.Total, len(page.Items), page.Page)
	}
}
