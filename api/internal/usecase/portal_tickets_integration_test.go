//go:build integration

// Tests d'isolation des tickets du portail.
//
// Bloquants : une note interne de l'equipe ne sort jamais vers le client — ni
// dans le fil, ni dans un e-mail — et un client ne voit ni les tickets internes
// de l'agence, ni ceux d'un autre client.
package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/domain"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

// mailsTo rend les sujets et corps des e-mails en file pour une adresse.
func mailsTo(t *testing.T, pool *pgxpool.Pool, address string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT subject || E'\n' || text_body || E'\n' || html_body FROM email_outbox WHERE to_address = $1 ORDER BY created_at`, address)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func newTicketPortal(t *testing.T, pool *pgxpool.Pool) (*usecase.PortalService, *usecase.TicketService) {
	t.Helper()
	_, deliverables, files := newPortal(t, pool)
	tickets := usecase.NewTicketService(pool, nil)
	tickets.SetMail("https://piilot.test", "https://piilot.test", true)

	return usecase.NewPortalService(pool, files, 1<<20, deliverables, tickets), tickets
}

func TestLesNotesInternesNeSortentJamaisDuPortail(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, tickets := newTicketPortal(t, pool)

	a := newPortalClient(t, pool)
	member, _ := createUser(t, pool, "team")

	ticket, err := portal.CreateTicket(ctx, a.userID, usecase.PortalTicketInput{
		ProjectID: a.projectID, Tracker: "anomalie", Subject: "Le formulaire ne part pas",
		Description: "Rien ne se passe au clic sur Envoyer.",
	})
	if err != nil {
		t.Fatalf("depot : %v", err)
	}

	const secret = "NOTE INTERNE : relancer la facture en retard"
	if _, err := tickets.PostMessage(ctx, ticket.ID, usecase.PostMessageInput{
		Body: secret, IsInternal: true, AuthorID: &member,
	}); err != nil {
		t.Fatal(err)
	}
	inProgress := "in_progress"
	if _, err := tickets.PostMessage(ctx, ticket.ID, usecase.PostMessageInput{
		Body: "Nous regardons, merci pour la capture.", AuthorID: &member, NewStatus: &inProgress,
	}); err != nil {
		t.Fatal(err)
	}
	done := "done"
	if _, err := tickets.PostMessage(ctx, ticket.ID, usecase.PostMessageInput{
		Body: secret + " (bis)", IsInternal: true, AuthorID: &member, NewStatus: &done,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := portal.Ticket(ctx, a.userID, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	var messages, statuses int
	for _, e := range got.Entries {
		if strings.Contains(e.Body, "NOTE INTERNE") {
			t.Fatalf("une note interne est exposee au client : %+v", e)
		}
		switch e.Kind {
		case "message":
			messages++
		case "status":
			statuses++
		}
	}
	if messages != 1 || statuses != 2 || got.Open {
		t.Fatalf("fil du client : %d messages, %d statuts, ouvert=%v — %+v", messages, statuses, got.Open, got.Entries)
	}

	// Deux e-mails : la reponse publique (avec son statut), puis la cloture,
	// sans un mot de la note interne qui la portait.
	mails := mailsTo(t, pool, a.email)
	if len(mails) != 2 {
		t.Fatalf("e-mails au client : %d, attendu 2 — %v", len(mails), mails)
	}
	for _, m := range mails {
		if strings.Contains(m, "NOTE INTERNE") {
			t.Fatalf("une note interne est partie par e-mail : %s", m)
		}
	}
	if !strings.Contains(mails[0], "Nous regardons") || !strings.Contains(mails[1], "Résolue") {
		t.Fatalf("contenu des e-mails : %v", mails)
	}
}

func TestUnClientNeVoitNiLesTicketsInternesNiCeuxDUnAutre(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, tickets := newTicketPortal(t, pool)

	a := newPortalClient(t, pool)
	b := newPortalClient(t, pool)
	member, _ := createUser(t, pool, "team")

	// Un ticket que l'equipe ouvre pour elle-meme sur le projet de A.
	internal, err := tickets.Create(ctx, usecase.CreateTicketInput{
		ProjectID: a.projectID, Subject: "Dette technique", Tracker: "evolution", Priority: "low", CreatedBy: &member,
	})
	if err != nil {
		t.Fatal(err)
	}
	ofB, err := portal.CreateTicket(ctx, b.userID, usecase.PortalTicketInput{
		ProjectID: b.projectID, Tracker: "assistance", Subject: "Question de B", Description: "Comment faire ?",
	})
	if err != nil {
		t.Fatal(err)
	}
	fileOfB, err := portal.AttachToTicket(ctx, b.userID, ofB.ID, "capture.png", "image/png", strings.NewReader("png"))
	if err != nil {
		t.Fatal(err)
	}

	list, err := portal.Tickets(ctx, a.userID, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("A voit des tickets qui ne sont pas les siens : %+v", list.Items)
	}

	for _, id := range []uuid.UUID{internal.ID, ofB.ID} {
		_, err := portal.Ticket(ctx, a.userID, id)
		mustNotFound(t, "A lit un ticket", err)
		_, err = portal.Reply(ctx, a.userID, id, "Je m'invite")
		mustNotFound(t, "A repond sur un ticket", err)
		_, err = portal.AttachToTicket(ctx, a.userID, id, "x.txt", "text/plain", strings.NewReader("x"))
		mustNotFound(t, "A joint un fichier a un ticket", err)
	}
	_, _, err = portal.OpenFile(ctx, a.userID, fileOfB.ID)
	mustNotFound(t, "A telecharge la piece jointe de B", err)

	// B, lui, la retrouve.
	_, content, err := portal.OpenFile(ctx, b.userID, fileOfB.ID)
	if err != nil {
		t.Fatalf("B telecharge sa piece jointe : %v", err)
	}
	content.Close()

	// Et A ne depose pas sur le projet de B.
	if _, err := portal.CreateTicket(ctx, a.userID, usecase.PortalTicketInput{
		ProjectID: b.projectID, Tracker: "anomalie", Subject: "x", Description: "x",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("A depose sur le projet de B : %v", err)
	}
}

func TestUneDemandeDuPortailPrevientLAgence(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	portal, tickets := newTicketPortal(t, pool)

	a := newPortalClient(t, pool)
	adminID, adminEmail := createUser(t, pool, "admin")
	member, memberEmail := createUser(t, pool, "team")

	if _, err := portal.CreateTicket(ctx, a.userID, usecase.PortalTicketInput{
		ProjectID: a.projectID, Tracker: "anomalie", Priority: "urgent", Subject: "x", Description: "x",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("priorite urgente acceptee depuis le portail : %v", err)
	}

	ticket, err := portal.CreateTicket(ctx, a.userID, usecase.PortalTicketInput{
		ProjectID: a.projectID, Tracker: "anomalie", Priority: "high", Subject: "Erreur 500 au paiement",
		Description: "La page de paiement renvoie une erreur.",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Personne ne traite encore le ticket : les administrateurs sont prevenus.
	if mails := mailsTo(t, pool, adminEmail); len(mails) != 1 || !strings.Contains(mails[0], "Nouvelle demande #") {
		t.Fatalf("e-mail aux administrateurs : %v", mails)
	}
	if kinds := notificationsOf(t, pool, adminID, "ticket_id", ticket.ID); len(kinds) != 1 || kinds[0] != usecase.NotifyTicketCreated {
		t.Fatalf("notification aux administrateurs : %v", kinds)
	}

	// Une fois confie, c'est la personne qui le traite qui recoit la reponse
	// du client, et non plus les administrateurs.
	todo := "todo"
	if _, err := tickets.PostMessage(ctx, ticket.ID, usecase.PostMessageInput{
		Body: "Je le prends.", IsInternal: true, AuthorID: &adminID, NewStatus: &todo,
		ChangeAssignee: true, NewAssigneeID: &member,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := portal.Reply(ctx, a.userID, ticket.ID, "Voici une capture supplémentaire."); err != nil {
		t.Fatal(err)
	}
	if mails := mailsTo(t, pool, memberEmail); len(mails) != 1 || !strings.Contains(mails[0], "Voici une capture") {
		t.Fatalf("e-mail a la personne qui traite : %v", mails)
	}
	if mails := mailsTo(t, pool, adminEmail); len(mails) != 1 {
		t.Fatalf("les administrateurs ne recoivent plus la suite : %d e-mails", len(mails))
	}
	if kinds := notificationsOf(t, pool, member, "ticket_id", ticket.ID); len(kinds) < 2 || kinds[len(kinds)-1] != usecase.NotifyTicketReplied {
		t.Fatalf("notification de la reponse : %v", kinds)
	}

	got, err := portal.Ticket(ctx, a.userID, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := got.Entries[len(got.Entries)-1]
	if last.Kind != "message" || last.FromAgency || last.Author != "Inès" {
		t.Fatalf("derniere entree : %+v", last)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM email_outbox WHERE to_address = ANY($1)`, []string{adminEmail, memberEmail})
	})
}
