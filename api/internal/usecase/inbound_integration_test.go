//go:build integration

package usecase_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/plugiit/piilot-app/api/internal/config"
	"github.com/plugiit/piilot-app/api/internal/mailin"
	"github.com/plugiit/piilot-app/api/internal/storage"
	"github.com/plugiit/piilot-app/api/internal/usecase"
)

const inboundSecret = "une-cle-de-test-assez-longue-pour-signer"

// newInbound monte l'e-mail entrant sur une adresse de support de test.
func newInbound(t *testing.T, pool *pgxpool.Pool) (*usecase.InboundService, *usecase.TicketService) {
	t.Helper()
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tickets := usecase.NewTicketService(pool, nil)
	tickets.SetMail("https://piilot.test", "https://piilot.test", "https://piilot.test", true)
	tickets.SetThreading([]byte(inboundSecret), "support@agence.test")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	inbound := usecase.NewInboundService(pool, tickets, files, 1<<20, nil,
		config.InboundEnv{Address: "support@agence.test"}, []byte(inboundSecret), "https://piilot.test", "piilot@agence.test", log)
	return inbound, tickets
}

// deliver depose un e-mail comme le ferait la releve, puis le range.
func deliver(t *testing.T, inbound *usecase.InboundService, d mailin.Draft) {
	t.Helper()
	if d.MessageID == "" {
		d.MessageID = uuid.NewString() + "@client.test"
	}
	raw, err := mailin.Compose(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inbound.Ingest(context.Background(), "raw", raw); err != nil {
		t.Fatal(err)
	}
	inbound.ProcessPending(context.Background())
}

func inboundOutcome(t *testing.T, pool *pgxpool.Pool, from string) (status, reason string, ticketID *uuid.UUID) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT status, reason, ticket_id FROM inbound_emails WHERE from_address = $1 ORDER BY received_at DESC, id DESC LIMIT 1`, from,
	).Scan(&status, &reason, &ticketID); err != nil {
		t.Fatalf("e-mail introuvable : %v", err)
	}
	return status, reason, ticketID
}

type outbox struct{ subject, text, replyTo, messageID, inReplyTo string }

func lastMail(t *testing.T, pool *pgxpool.Pool, to string) outbox {
	t.Helper()
	var o outbox
	if err := pool.QueryRow(context.Background(),
		`SELECT subject, text_body, reply_to, message_id, in_reply_to FROM email_outbox WHERE to_address = $1 ORDER BY created_at DESC LIMIT 1`, to,
	).Scan(&o.subject, &o.text, &o.replyTo, &o.messageID, &o.inReplyTo); err != nil {
		t.Fatalf("aucun e-mail a %s : %v", to, err)
	}
	return o
}

func cleanupInbound(t *testing.T, pool *pgxpool.Pool, addresses ...string) {
	t.Cleanup(func() {
		for _, a := range addresses {
			_, _ = pool.Exec(context.Background(), `DELETE FROM inbound_emails WHERE from_address = $1`, a)
			_, _ = pool.Exec(context.Background(), `DELETE FROM email_outbox WHERE to_address = $1`, a)
		}
	})
}

func TestUnClientOuvreEtSuitUnTicketParEmail(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	inbound, tickets := newInbound(t, pool)
	a := newPortalClient(t, pool)
	cleanupInbound(t, pool, a.email)

	// Un e-mail du client, sur son seul projet actif : un ticket.
	deliver(t, inbound, mailin.Draft{
		From: mailin.Address{Name: "Inès Client", Address: a.email}, To: []mailin.Address{{Address: "support@agence.test"}},
		Subject: "TR: Le formulaire de contact ne part plus", Text: "Bonjour,\n\nLe formulaire renvoie une erreur.\n\nInès",
		Attachments: []mailin.Attachment{{Filename: "capture.png", ContentType: "image/png", Content: []byte("png")}},
	})
	status, reason, ticketID := inboundOutcome(t, pool, a.email)
	if status != "processed" || reason != "created" || ticketID == nil {
		t.Fatalf("ouverture : %s %s %v", status, reason, ticketID)
	}
	detail, err := tickets.Get(ctx, *ticketID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Subject != "Le formulaire de contact ne part plus" || detail.Project.ID != a.projectID ||
		!detail.ClientVisible || detail.Reporter == nil || len(detail.Files) != 1 || detail.Tracker != "assistance" {
		t.Fatalf("ticket : %+v", detail)
	}

	// L'accuse de reception part dans le fil, avec une adresse de reponse signee.
	ack := lastMail(t, pool, a.email)
	if !strings.Contains(ack.subject, "bien reçue") || !strings.HasPrefix(ack.replyTo, "support+t") ||
		!strings.HasSuffix(ack.replyTo, "@agence.test") || ack.inReplyTo == "" {
		t.Fatalf("accuse de reception : %+v", ack)
	}

	// Le client repond a l'accuse : sa reponse rejoint le ticket, sans la
	// citation.
	deliver(t, inbound, mailin.Draft{
		From: mailin.Address{Address: a.email}, To: []mailin.Address{{Address: ack.replyTo}},
		InReplyTo: ack.messageID, Subject: "Re: " + ack.subject,
		Text: "Il manque aussi le bouton d'envoi.\n\nLe lun. 7 oct. 2026 à 10:02, Piilot <support@agence.test> a écrit :\n> Nous avons bien reçu",
	})
	if s, r, id := inboundOutcome(t, pool, a.email); s != "processed" || r != "replied" || *id != *ticketID {
		t.Fatalf("reponse : %s %s", s, r)
	}
	detail, _ = tickets.Get(ctx, *ticketID)
	last := detail.Entries[len(detail.Entries)-1]
	if last.Body != "Il manque aussi le bouton d'envoi." || !last.ViaEmail || last.Author == nil || last.Author.ID != a.userID || last.IsInternal {
		t.Fatalf("message par e-mail : %+v", last)
	}

	// La reponse de l'agence part filee, et le client clot puis rouvre par e-mail.
	member, _ := createUser(t, pool, "team")
	done := "done"
	if _, err := tickets.PostMessage(ctx, *ticketID, usecase.PostMessageInput{Body: "Corrigé.", AuthorID: &member, NewStatus: &done}); err != nil {
		t.Fatal(err)
	}
	answer := lastMail(t, pool, a.email)
	if !strings.Contains(answer.text, "Corrigé.") || answer.replyTo != ack.replyTo {
		t.Fatalf("reponse de l'agence : %+v", answer)
	}
	deliver(t, inbound, mailin.Draft{
		From: mailin.Address{Address: a.email}, To: []mailin.Address{{Address: "support@agence.test"}},
		References: answer.messageID, Subject: "Re: x", Text: "Toujours en panne chez moi.",
	})
	detail, _ = tickets.Get(ctx, *ticketID)
	if detail.Status != "todo" {
		t.Fatalf("un ticket clos doit rouvrir : %s", detail.Status)
	}

	// Une reponse automatique ne range rien ; un e-mail deja recu n'entre pas.
	deliver(t, inbound, mailin.Draft{
		From: mailin.Address{Address: a.email}, To: []mailin.Address{{Address: ack.replyTo}},
		Subject: "Absent", Text: "Je suis absent", Headers: map[string]string{"Auto-Submitted": "auto-replied"},
	})
	if s, r, _ := inboundOutcome(t, pool, a.email); s != "ignored" || r != "auto_reply" {
		t.Fatalf("reponse automatique : %s %s", s, r)
	}
	raw, _ := mailin.Compose(mailin.Draft{MessageID: "deja@client.test", From: mailin.Address{Address: a.email}, Text: "x"})
	if ok, _ := inbound.Ingest(ctx, "raw", raw); !ok {
		t.Fatal("premier depot refuse")
	}
	if ok, _ := inbound.Ingest(ctx, "imap", raw); ok {
		t.Fatal("le meme e-mail est entre deux fois")
	}

	// Une etiquette forgee ne mene nulle part.
	forged := strings.Replace(ack.replyTo, ack.replyTo[strings.Index(ack.replyTo, ".")+1:strings.Index(ack.replyTo, "@")], "0000000000", 1)
	deliver(t, inbound, mailin.Draft{
		From: mailin.Address{Address: a.email}, To: []mailin.Address{{Address: forged}}, Subject: "Re: x", Text: "Forge",
	})
	if s, r, id := inboundOutcome(t, pool, a.email); s != "processed" || r != "created" || *id == *ticketID {
		t.Fatalf("etiquette forgee : %s %s", s, r)
	}
}

func TestUnContactSansCompteEtLeTri(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	inbound, tickets := newInbound(t, pool)

	projectID := newProject(t, pool, 0)
	clientID := clientOf(t, pool, projectID)
	contact := "contact-" + uuid.NewString()[:8] + "@client.test"
	stranger := "inconnu-" + uuid.NewString()[:8] + "@ailleurs.test"
	cleanupInbound(t, pool, contact, stranger)
	if _, err := pool.Exec(ctx, `INSERT INTO contacts (client_id, firstname, lastname, email) VALUES ($1, 'Paul', 'Contact', $2)`, clientID, contact); err != nil {
		t.Fatal(err)
	}

	// Un contact du CRM, sans compte : le ticket garde son adresse.
	deliver(t, inbound, mailin.Draft{From: mailin.Address{Address: contact}, To: []mailin.Address{{Address: "support@agence.test"}},
		Subject: "Site lent", Text: "Le site met dix secondes à charger."})
	_, reason, ticketID := inboundOutcome(t, pool, contact)
	if reason != "created" {
		t.Fatalf("contact : %s", reason)
	}
	detail, _ := tickets.Get(ctx, *ticketID)
	if detail.Requester == nil || detail.Requester.Email != contact || detail.Requester.Name != "Paul Contact" || detail.Reporter != nil {
		t.Fatalf("demandeur : %+v / %+v", detail.Requester, detail.Reporter)
	}
	member, _ := createUser(t, pool, "team")
	if _, err := tickets.PostMessage(ctx, *ticketID, usecase.PostMessageInput{Body: "On regarde.", AuthorID: &member}); err != nil {
		t.Fatal(err)
	}
	if m := lastMail(t, pool, contact); !strings.Contains(m.text, "On regarde.") || strings.Contains(m.text, "/client/tickets/") || m.replyTo == "" {
		t.Fatalf("reponse au contact sans compte : %+v", m)
	}

	// Un second projet actif : l'e-mail suivant attend qu'on choisisse.
	var second uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO projects (client_id, name, status) VALUES ($1, $2, 'hebergement') RETURNING id`,
		clientID, "Hébergement "+uuid.NewString()[:6]).Scan(&second); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id = $1`, second) })
	deliver(t, inbound, mailin.Draft{From: mailin.Address{Address: contact}, Subject: "Certificat", Text: "Le certificat expire."})
	status, reason, _ := inboundOutcome(t, pool, contact)
	if status != "held" || reason != "project_to_choose" {
		t.Fatalf("deux projets : %s %s", status, reason)
	}
	page, err := inbound.Held(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var held usecase.InboundItem
	for _, it := range page.Items {
		if it.FromAddress == contact {
			held = it
		}
	}
	if held.Client == nil || held.Client.ID != clientID || len(held.Projects) != 2 || held.Excerpt != "Le certificat expire." {
		t.Fatalf("a trier : %+v", held)
	}
	opened, err := inbound.OpenTicket(ctx, held.ID, second)
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := tickets.Get(ctx, opened); d.Project.ID != second || d.Requester == nil {
		t.Fatalf("ticket trie : %+v", d)
	}
	if _, err := inbound.OpenTicket(ctx, held.ID, second); err == nil {
		t.Fatal("un e-mail deja trie a ete trie deux fois")
	}

	// Un inconnu attend ; on l'ajoute a un ticket, ou on l'ecarte.
	deliver(t, inbound, mailin.Draft{From: mailin.Address{Address: stranger}, Subject: "Question", Text: "Une question."})
	if s, r, _ := inboundOutcome(t, pool, stranger); s != "held" || r != "unknown_sender" {
		t.Fatalf("inconnu : %s %s", s, r)
	}
	var strangerID uuid.UUID
	_ = pool.QueryRow(ctx, `SELECT id FROM inbound_emails WHERE from_address = $1`, stranger).Scan(&strangerID)
	numero := detail.Numero
	if _, err := inbound.AttachToTicket(ctx, strangerID, numero); err != nil {
		t.Fatal(err)
	}
	d, _ := tickets.Get(ctx, *ticketID)
	last := d.Entries[len(d.Entries)-1]
	if last.Body != "Une question." || last.SenderMail != stranger || last.Author != nil || last.IsInternal {
		t.Fatalf("ajout au ticket : %+v", last)
	}

	deliver(t, inbound, mailin.Draft{From: mailin.Address{Address: stranger}, Subject: "Pub", Text: "Achetez."})
	_ = pool.QueryRow(ctx, `SELECT id FROM inbound_emails WHERE from_address = $1 AND status = 'held'`, stranger).Scan(&strangerID)
	if err := inbound.Dismiss(ctx, strangerID); err != nil {
		t.Fatal(err)
	}
	if s, r, _ := inboundOutcome(t, pool, stranger); s != "ignored" || r != "dismissed" {
		t.Fatalf("ecarte : %s %s", s, r)
	}
}

func TestLeJournalDAuditEstEnAjoutSeul(t *testing.T) {
	_, pool := newService(t)
	ctx := context.Background()
	audit := usecase.NewAuditService(pool, 365*24*time.Hour)
	actor, _ := createUser(t, pool, "admin")
	marker := uuid.NewString()

	if err := audit.Record(ctx, usecase.AuditEntry{ActorID: &actor, Action: "test.action", TargetID: marker, IP: "203.0.113.7",
		Details: map[string]any{"role": "team"}}); err != nil {
		t.Fatal(err)
	}
	page, err := audit.List(ctx, usecase.AuditFilters{Action: "test.", Search: marker}, 1)
	if err != nil || page.Total != 1 || page.Items[0].ActorEmail == "" || page.Items[0].Details["role"] != "team" {
		t.Fatalf("lecture : %+v %v", page, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE audit_log SET action = 'x' WHERE target_id = $1`, marker); err == nil {
		t.Fatal("une ligne d'audit a ete modifiee")
	}
	if _, err := pool.Exec(ctx, `UPDATE audit_log SET actor_id = NULL, actor_email = '' WHERE target_id = $1`, marker); err == nil {
		t.Fatal("l'adresse figee a ete effacee")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM audit_log WHERE target_id = $1`, marker); err == nil {
		t.Fatal("une ligne d'audit a ete effacee hors purge")
	}

	// La purge efface ce qui depasse la retention, et seulement cela.
	old := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO audit_log (action, target_id, at) VALUES ('test.old', $1, now() - interval '400 days')`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := audit.Purge(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE target_id IN ($1, $2)`, old, marker).Scan(&left)
	if left != 1 {
		t.Fatalf("apres purge : %d lignes", left)
	}
}
