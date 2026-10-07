//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

func TestLeLogoDUnProjetSeDeposeSeRemplaceEtSeRetire(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	author, _ := createUser(t, pool, "admin")

	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projects := usecase.NewProjectService(pool, files, 1<<20)

	project, err := projects.Create(ctx, usecase.CreateProjectInput{
		Name: uniqueName("Projet logo"), ClientName: uniqueName("Client logo"), CreatedBy: author,
	})
	if err != nil {
		t.Fatalf("creation du projet : %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, project.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM clients WHERE id = $1`, project.ClientID)
	})
	if project.LogoURL != nil {
		t.Fatalf("un projet neuf n'a pas de logo : %v", *project.LogoURL)
	}

	var derr *domain.Error
	_, err = projects.SetLogoFile(ctx, project.ID, author, "application/pdf", strings.NewReader("%PDF"))
	if !errors.As(err, &derr) || derr.Details["file"] == nil {
		t.Fatalf("un PDF a ete accepte comme logo : %v", err)
	}

	svg := `<svg xmlns="http://www.w3.org/2000/svg"><circle r="4"/></svg>`
	first, err := projects.SetLogoFile(ctx, project.ID, author, "image/svg+xml; charset=utf-8", strings.NewReader(svg))
	if err != nil {
		t.Fatalf("depot : %v", err)
	}
	if first.LogoURL == nil || !strings.HasPrefix(*first.LogoURL, "/api/v1/auth/project-logos/") {
		t.Fatalf("adresse du logo : %v", first.LogoURL)
	}
	firstKey := strings.TrimPrefix(*first.LogoURL, "/api/v1/auth/project-logos/")

	content, err := projects.OpenLogo(ctx, firstKey)
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	data, _ := io.ReadAll(content)
	_ = content.Close()
	if string(data) != svg {
		t.Fatalf("contenu relu : %q", data)
	}

	// Une cle qui n'est pas un logo — une piece jointe, par exemple — ne passe
	// pas par cette porte.
	if _, err := projects.OpenLogo(ctx, "pas-un-logo"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cle etrangere servie : %v", err)
	}

	// Remplacer efface l'ancien fichier.
	second, err := projects.SetLogoFile(ctx, project.ID, author, "image/png", strings.NewReader("\x89PNG\r\n\x1a\n"))
	if err != nil {
		t.Fatalf("remplacement : %v", err)
	}
	if *second.LogoURL == *first.LogoURL {
		t.Fatal("le remplacement a garde la meme adresse")
	}
	if _, err := files.Open(firstKey); err == nil {
		t.Fatal("l'ancien logo n'a pas ete efface")
	}

	// Le logo suit le projet dans les listes.
	page, err := projects.List(ctx, usecase.ProjectFilters{Viewer: author, Page: 1, PageSize: 50, Search: &project.Name})
	if err != nil {
		t.Fatalf("liste : %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].LogoURL == nil || *page.Items[0].LogoURL != *second.LogoURL {
		t.Fatalf("logo absent de la liste : %+v", page.Items)
	}

	removed, err := projects.RemoveLogo(ctx, project.ID, author)
	if err != nil {
		t.Fatalf("retrait : %v", err)
	}
	if removed.LogoURL != nil {
		t.Fatalf("logo encore la apres retrait : %v", *removed.LogoURL)
	}
	if _, err := projects.OpenLogo(ctx, strings.TrimPrefix(*second.LogoURL, "/api/v1/auth/project-logos/")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("logo retire encore servi : %v", err)
	}
}
