//go:build integration

// Tests d'isolation du portail client.
//
// Ils sont bloquants : un compte client ne lit, ne modifie et ne devine jamais
// une donnee d'un autre client. Chaque test monte deux clients et verifie que
// l'un ne voit rien de l'autre — un identifiant etranger rend « introuvable »,
// exactement comme un identifiant qui n'existe pas.
package usecase_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// portalFixture : un client, un projet, un compte du portail.
type portalFixture struct {
	clientID  uuid.UUID
	projectID uuid.UUID
	userID    uuid.UUID
	email     string
}

func newPortalClient(t *testing.T, pool *pgxpool.Pool) portalFixture {
	t.Helper()
	ctx := context.Background()

	projectID := newProject(t, pool, 0)
	clientID := clientOf(t, pool, projectID)

	email := "portail-" + uuid.NewString() + "@piilot.test"
	var userID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, firstname, lastname, role, client_id)
		 VALUES ($1, 'x', 'Inès', 'Client', 'client', $2) RETURNING id`, email, clientID,
	).Scan(&userID); err != nil {
		t.Fatalf("compte du portail : %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM email_outbox WHERE to_address = $1`, email)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	return portalFixture{clientID: clientID, projectID: projectID, userID: userID, email: email}
}

// newPortal monte le service du portail sur un stockage jetable.
func newPortal(t *testing.T, pool *pgxpool.Pool) (*usecase.PortalService, *usecase.DeliverableService, storage.Store) {
	t.Helper()
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deliverables := usecase.NewDeliverableService(pool, nil)

	return usecase.NewPortalService(pool, files, deliverables), deliverables, files
}

// saveFile depose un fichier de projet, partage ou non.
func saveFile(t *testing.T, pool *pgxpool.Pool, files storage.Store, projectID uuid.UUID, shared bool) uuid.UUID {
	t.Helper()
	key, size, err := files.Save(strings.NewReader("contenu du fichier"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO attachments (project_id, filename, content_type, size_bytes, storage_key, shared_with_client)
		 VALUES ($1, 'devis.pdf', 'application/pdf', $2, $3, $4) RETURNING id`, projectID, size, key, shared,
	).Scan(&id); err != nil {
		t.Fatalf("fichier : %v", err)
	}
	return id
}

func mustNotFound(t *testing.T, what string, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("%s : erreur %v, attendu introuvable", what, err)
	}
}

func TestLePortailNeMontreQueLesProjetsDuClient(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, _, _ := newPortal(t, pool)

	a := newPortalClient(t, pool)
	b := newPortalClient(t, pool)

	// Un projet interne rattache au meme client n'existe pas pour le portail.
	var internal uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (client_id, name, status, is_internal) VALUES ($1, 'Interne', 'production', true) RETURNING id`,
		a.clientID,
	).Scan(&internal); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, internal) })

	list, err := portal.Projects(ctx, a.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != a.projectID {
		t.Fatalf("projets vus par A : %+v", list.Items)
	}

	if _, err := portal.Project(ctx, a.userID, a.projectID); err != nil {
		t.Fatalf("A lit son projet : %v", err)
	}
	_, err = portal.Project(ctx, a.userID, b.projectID)
	mustNotFound(t, "A lit le projet de B", err)
	_, err = portal.Project(ctx, a.userID, internal)
	mustNotFound(t, "A lit le projet interne", err)

	// Un compte de l'agence n'a pas de client : le portail ne lui montre rien.
	staff, _ := createUser(t, pool, "admin")
	empty, err := portal.Projects(ctx, staff)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Items) != 0 {
		t.Fatalf("un compte interne voit %d projets", len(empty.Items))
	}
	_, err = portal.Project(ctx, staff, a.projectID)
	mustNotFound(t, "un compte interne lit un projet par le portail", err)

	// Un compte desactive ne voit plus rien, meme avec une session ouverte.
	if _, err := pool.Exec(ctx, `UPDATE users SET disabled_at = now() WHERE id = $1`, a.userID); err != nil {
		t.Fatal(err)
	}
	_, err = portal.Project(ctx, a.userID, a.projectID)
	mustNotFound(t, "compte desactive", err)
}

func TestUnClientNeTouchePasAuxLivrablesNiAuxFichiersDUnAutre(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, deliverables, files := newPortal(t, pool)
	staff, _ := createUser(t, pool, "team")

	a := newPortalClient(t, pool)
	b := newPortalClient(t, pool)

	dB, err := deliverables.Create(ctx, usecase.CreateDeliverableInput{
		ProjectID: b.projectID, Title: "Maquettes B", URL: "https://example.fr/b", CreatedBy: &staff,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = portal.Deliverable(ctx, a.userID, dB.ID)
	mustNotFound(t, "A lit le livrable de B", err)
	_, err = portal.Decide(ctx, a.userID, dB.ID, "valide", "")
	mustNotFound(t, "A valide le livrable de B", err)

	// La tentative n'a rien laisse : la version de B attend toujours.
	got, err := portal.Deliverable(ctx, b.userID, dB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "en_attente" {
		t.Fatalf("le livrable de B a change d'etat : %s", got.Status)
	}

	sharedB := saveFile(t, pool, files, b.projectID, true)
	internalA := saveFile(t, pool, files, a.projectID, false)
	sharedA := saveFile(t, pool, files, a.projectID, true)

	_, _, err = portal.OpenFile(ctx, a.userID, sharedB)
	mustNotFound(t, "A telecharge un fichier partage de B", err)
	_, _, err = portal.OpenFile(ctx, a.userID, internalA)
	mustNotFound(t, "A telecharge un fichier interne de son projet", err)

	_, content, err := portal.OpenFile(ctx, a.userID, sharedA)
	if err != nil {
		t.Fatalf("A telecharge un fichier partage : %v", err)
	}
	body, _ := io.ReadAll(content)
	content.Close()
	if string(body) != "contenu du fichier" {
		t.Fatalf("contenu : %q", body)
	}

	detail, err := portal.Project(ctx, a.userID, a.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Files) != 1 || detail.Files[0].ID != sharedA {
		t.Fatalf("fichiers montres a A : %+v", detail.Files)
	}
}

func TestUnBrouillonResteInterne(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, _, _ := newPortal(t, pool)
	a := newPortalClient(t, pool)

	var draft uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO deliverables (project_id, title) VALUES ($1, 'Brouillon') RETURNING id`, a.projectID,
	).Scan(&draft); err != nil {
		t.Fatal(err)
	}

	detail, err := portal.Project(ctx, a.userID, a.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Deliverables) != 0 {
		t.Fatalf("un brouillon est montre au client : %+v", detail.Deliverables)
	}
	_, err = portal.Deliverable(ctx, a.userID, draft)
	mustNotFound(t, "le client lit un brouillon", err)
}

func TestLaReponseDuClientPrevientLEquipeEtNeSeRejouePas(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, deliverables, _ := newPortal(t, pool)
	deliverables.SetMail("https://piilot.test", true)

	a := newPortalClient(t, pool)
	member, _ := createUser(t, pool, "team")
	addMember(t, pool, a.projectID, member)

	item, err := deliverables.Create(ctx, usecase.CreateDeliverableInput{
		ProjectID: a.projectID, Title: "Page d'accueil", URL: "https://example.fr/v1", CreatedBy: &member,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Le client est prevenu par e-mail du livrable qui l'attend.
	var subject, text string
	if err := pool.QueryRow(ctx,
		`SELECT subject, text_body FROM email_outbox WHERE to_address = $1 ORDER BY created_at DESC LIMIT 1`, a.email,
	).Scan(&subject, &text); err != nil {
		t.Fatalf("e-mail au client : %v", err)
	}
	if !strings.Contains(subject, "Page d'accueil") || !strings.Contains(text, "/client/livrables/"+item.ID.String()) {
		t.Fatalf("e-mail : %q / %q", subject, text)
	}

	// Des retours sans un mot sont refuses.
	if _, err := portal.Decide(ctx, a.userID, item.ID, "retours", "  "); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("retours sans commentaire : %v", err)
	}

	got, err := portal.Decide(ctx, a.userID, item.ID, "retours", "Le logo est trop petit")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "retours" || got.Versions[0].Feedback != "Le logo est trop petit" || got.Versions[0].DecidedBy != "Inès" {
		t.Fatalf("apres retours : %+v", got)
	}
	if kinds := notificationsOf(t, pool, member, "deliverable_id", item.ID); len(kinds) != 1 || kinds[0] != usecase.NotifyDeliverableFeedback {
		t.Fatalf("notification de l'equipe : %v", kinds)
	}

	// Une seconde reponse sur la meme version est refusee, sans notifier.
	if _, err := portal.Decide(ctx, a.userID, item.ID, "valide", ""); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("seconde reponse : %v", err)
	}
	if kinds := notificationsOf(t, pool, member, "deliverable_id", item.ID); len(kinds) != 1 {
		t.Fatalf("la seconde reponse a notifie : %v", kinds)
	}

	// La v2 repart en attente, et le client en est prevenu.
	if _, err := deliverables.Submit(ctx, item.ID, "https://example.fr/v2", &member); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT subject FROM email_outbox WHERE to_address = $1 ORDER BY created_at DESC LIMIT 1`, a.email,
	).Scan(&subject); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject, "Nouvelle version") {
		t.Fatalf("e-mail de la v2 : %q", subject)
	}
	got, err = portal.Decide(ctx, a.userID, item.ID, "valide", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "valide" || len(got.Versions) != 2 || got.Versions[0].Numero != 2 {
		t.Fatalf("apres validation de la v2 : %+v", got)
	}
}
