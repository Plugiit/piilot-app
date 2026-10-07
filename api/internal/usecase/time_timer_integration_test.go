//go:build integration

package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func TestLeChronoDevientUneSaisieALArret(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	author, _ := createUser(t, pool, "team")

	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projects := usecase.NewProjectService(pool, files, 1<<20)
	project, err := projects.Create(ctx, usecase.CreateProjectInput{
		Name: uniqueName("Projet chrono"), ClientName: uniqueName("Client chrono"), CreatedBy: author,
	})
	if err != nil {
		t.Fatalf("creation du projet : %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM time_entries WHERE project_id = $1`, project.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, project.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, project.ClientID)
	})

	entries := usecase.NewTimeEntryService(pool)
	timers := usecase.NewTimerService(pool, entries)

	if current, err := timers.Current(ctx, author); err != nil || current != nil {
		t.Fatalf("avant demarrage : %v, %v", current, err)
	}

	started, err := timers.Start(ctx, author, usecase.TimerInput{ProjectID: project.ID, Note: "Recette"})
	if err != nil {
		t.Fatalf("demarrage : %v", err)
	}
	if started == nil || started.Project.ID != project.ID || started.Task != nil {
		t.Fatalf("chrono lance : %+v", started)
	}

	// Redemarrer sur la meme chose arrete le precedent et ecrit son temps :
	// un seul chrono, jamais deux.
	if _, err := timers.Start(ctx, author, usecase.TimerInput{ProjectID: project.ID}); err != nil {
		t.Fatalf("redemarrage : %v", err)
	}

	// On avance l'horloge de trente-sept minutes pour l'arret.
	if _, err := pool.Exec(ctx, `UPDATE time_timers SET started_at = now() - interval '37 minutes 10 seconds' WHERE user_id = $1`, author); err != nil {
		t.Fatal(err)
	}

	entry, err := timers.Stop(ctx, author)
	if err != nil {
		t.Fatalf("arret : %v", err)
	}
	if entry == nil || entry.Minutes != 38 || entry.Project.ID != project.ID {
		t.Fatalf("saisie ecrite : %+v", entry)
	}
	if entry.SpentOn != time.Now().Format(time.DateOnly) {
		t.Errorf("jour de la saisie : %s", entry.SpentOn)
	}

	if current, err := timers.Current(ctx, author); err != nil || current != nil {
		t.Fatalf("apres arret : %v, %v", current, err)
	}

	// Rien ne tourne : l'arret ne fait rien, sans erreur.
	if entry, err := timers.Stop(ctx, author); err != nil || entry != nil {
		t.Fatalf("second arret : %v, %v", entry, err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM time_entries WHERE project_id = $1`, project.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("%d saisies, attendu 2 (le redemarrage puis l'arret)", count)
	}
}
