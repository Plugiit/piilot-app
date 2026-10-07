//go:build integration

package usecase_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func TestUnePullRequestFaitAvancerTicketsEtTachesPuisLaMiseEnLigneLesClot(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	author, _ := createUser(t, pool, "team")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projects := usecase.NewProjectService(pool, files, 1<<20)
	tickets := usecase.NewTicketService(pool, nil)
	tasks := usecase.NewTaskService(pool, files, 1<<20, nil)
	milestones := usecase.NewMilestoneService(pool)
	git := usecase.NewGitService(pool, tickets, milestones, "", "https://piilot.test", log)

	project, err := projects.Create(ctx, usecase.CreateProjectInput{
		Name: uniqueName("Projet git"), ClientName: uniqueName("Client git"), CreatedBy: author,
		RepoURL: "git@github.com:Agence/Site-Client.git",
	})
	if err != nil {
		t.Fatalf("creation du projet : %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, project.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, project.ClientID)
	})
	if project.RepoURL != "https://github.com/agence/site-client" {
		t.Fatalf("depot normalise : %q", project.RepoURL)
	}

	ticket, err := tickets.Create(ctx, usecase.CreateTicketInput{
		ProjectID: project.ID, Subject: "Le panier se vide", Tracker: "anomalie", Priority: "high", CreatedBy: &author,
	})
	if err != nil {
		t.Fatalf("creation du ticket : %v", err)
	}
	task, err := tasks.Create(ctx, usecase.CreateTaskInput{ProjectID: project.ID, Title: "Corriger le panier", ActorID: author})
	if err != nil {
		t.Fatalf("creation de la tache : %v", err)
	}
	title := "Mise en ligne"
	milestone, err := milestones.Create(ctx, project.ID, usecase.MilestoneInput{Title: &title}, author)
	if err != nil {
		t.Fatalf("creation du jalon : %v", err)
	}

	repo := map[string]any{"html_url": "https://github.com/Agence/Site-Client"}
	pr := func(action string, merged bool, state string) []byte {
		body, _ := json.Marshal(map[string]any{
			"action": action,
			"pull_request": map[string]any{
				"number": 12, "title": fmt.Sprintf("Corrige le panier #%d", ticket.Numero),
				"body": "Voir aussi T-" + fmt.Sprint(task.Numero), "html_url": "https://github.com/Agence/Site-Client/pull/12",
				"state": state, "merged": merged, "head": map[string]any{"ref": "fix/panier"}, "user": map[string]any{"login": "max"},
			},
			"repository": repo,
		})
		return body
	}

	// Ouverture : rattachee, ticket en revue, tache en revue.
	out, err := git.HandleGitHub(ctx, "pull_request", pr("opened", false, "open"))
	if err != nil || !out.Handled {
		t.Fatalf("ouverture : %+v, %v", out, err)
	}
	gotTicket, _ := tickets.Get(ctx, ticket.ID)
	gotTask, _ := tasks.Get(ctx, task.ID)
	if gotTicket.Status != "in_review" || len(gotTicket.PullRequests) != 1 || gotTicket.PullRequests[0].Number != 12 {
		t.Fatalf("ticket apres ouverture : %s, %+v", gotTicket.Status, gotTicket.PullRequests)
	}
	if gotTask.Status != "review" || len(gotTask.PullRequests) != 1 {
		t.Fatalf("tache apres ouverture : %s, %+v", gotTask.Status, gotTask.PullRequests)
	}

	// Fusion : ticket pret a deployer, tache terminee. Le meme evenement deux
	// fois ne fait rien de plus.
	for range 2 {
		if _, err := git.HandleGitHub(ctx, "pull_request", pr("closed", true, "closed")); err != nil {
			t.Fatalf("fusion : %v", err)
		}
	}
	gotTicket, _ = tickets.Get(ctx, ticket.ID)
	gotTask, _ = tasks.Get(ctx, task.ID)
	if gotTicket.Status != "ready_to_deploy" || gotTicket.PullRequests[0].State != "merged" {
		t.Fatalf("ticket apres fusion : %s, %+v", gotTicket.Status, gotTicket.PullRequests)
	}
	if gotTask.Status != "done" {
		t.Fatalf("tache apres fusion : %s", gotTask.Status)
	}

	// Un depot inconnu ne touche a rien.
	unknown, _ := json.Marshal(map[string]any{"action": "opened", "pull_request": map[string]any{"number": 1, "title": "#1", "state": "open", "head": map[string]any{}, "user": map[string]any{}}, "repository": map[string]any{"html_url": "https://github.com/autre/depot"}})
	if out, err := git.HandleGitHub(ctx, "pull_request", unknown); err != nil || out.Handled {
		t.Fatalf("depot inconnu : %+v, %v", out, err)
	}

	// Mise en ligne : ticket clos avec un mot au client, jalon atteint,
	// journal du client.
	release, _ := json.Marshal(map[string]any{
		"action": "published", "release": map[string]any{"tag_name": "v1.2.0", "html_url": "https://github.com/Agence/Site-Client/releases/tag/v1.2.0"},
		"repository": repo,
	})
	out, err = git.HandleGitHub(ctx, "release", release)
	if err != nil || !out.Handled {
		t.Fatalf("release : %+v, %v", out, err)
	}
	gotTicket, _ = tickets.Get(ctx, ticket.ID)
	if gotTicket.Status != "done" {
		t.Fatalf("ticket apres mise en ligne : %s", gotTicket.Status)
	}
	told := false
	for _, entry := range gotTicket.Entries {
		if entry.Kind == "message" && !entry.IsInternal && entry.Body == "Mis en ligne avec v1.2.0." {
			told = true
		}
	}
	if !told {
		t.Fatalf("le client doit lire la mise en ligne : %+v", gotTicket.Entries)
	}
	list, _ := milestones.List(ctx, project.ID)
	for _, m := range list.Items {
		if m.ID == milestone.ID && m.State != "done" {
			t.Fatalf("jalon de mise en ligne non atteint : %+v", m)
		}
	}
	if kinds := interactionKinds(t, pool, project.ClientID); kinds[0] != usecase.InteractionDeployment {
		t.Fatalf("journal du client : %v", kinds)
	}

	// La meme release une seconde fois : deja connue, rien ne change.
	if out, err := git.HandleGitHub(ctx, "release", release); err != nil || !out.Handled || out.Note != "mise en ligne déjà connue" {
		t.Fatalf("release rejouee : %+v, %v", out, err)
	}
}
