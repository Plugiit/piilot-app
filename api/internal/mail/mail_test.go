package mail

import (
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestInvitationPorteLeLienEtLesNoms(t *testing.T) {
	expires := time.Date(2026, 10, 9, 12, 30, 0, 0, time.UTC)
	msg, err := Invitation("lea@example.fr", "Léa", "Camille Laurent", "Atelier <Lumen>", "https://piilot.example.fr/invitation/abc", expires)
	if err != nil {
		t.Fatalf("Invitation : %v", err)
	}

	for _, want := range []string{"Bonjour Léa", "Camille Laurent vous invite", "https://piilot.example.fr/invitation/abc", "9 octobre 2026 à 14 h 30"} {
		if !strings.Contains(msg.Text, want) {
			t.Errorf("texte sans %q :\n%s", want, msg.Text)
		}
	}
	// Le HTML echappe ce qui vient de la base : un nom de client ne doit pas
	// pouvoir injecter de balise.
	if strings.Contains(msg.HTML, "<Lumen>") || !strings.Contains(msg.HTML, "&lt;Lumen&gt;") {
		t.Error("le nom du client n'est pas echappe dans le HTML")
	}
}

func TestPasswordResetSansPrenom(t *testing.T) {
	msg, err := PasswordReset("x@example.fr", "", "https://p.test/r/t", time.Now())
	if err != nil {
		t.Fatalf("PasswordReset : %v", err)
	}
	if !strings.HasPrefix(msg.Text, "Bonjour,") {
		t.Errorf("salutation : %q", msg.Text[:20])
	}
}

func TestBuildProduitUnMessageMIMELisible(t *testing.T) {
	from, _ := mail.ParseAddress("Piilot <piilot@example.fr>")
	raw, err := build(from, Message{To: "lea@example.fr", Subject: "Réinitialisation", Text: "texte", HTML: "<p>html</p>"})
	if err != nil {
		t.Fatalf("build : %v", err)
	}

	parsed, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("message illisible : %v", err)
	}

	subject, _ := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if subject != "Réinitialisation" {
		t.Errorf("sujet = %q", subject)
	}
	if !strings.HasPrefix(parsed.Header.Get("Content-Type"), "multipart/alternative") {
		t.Errorf("type = %q", parsed.Header.Get("Content-Type"))
	}
}

func TestValidateRefuseUnExpediteurInvalide(t *testing.T) {
	if err := (SMTP{Host: "smtp.test", From: "pas une adresse", Security: SecurityStartTLS}).Validate(); err == nil {
		t.Error("expediteur invalide accepte")
	}
	if err := (SMTP{}).Validate(); err != nil {
		t.Errorf("SMTP non configure : %v", err)
	}
}

func TestDeliverableSubmittedDitLaVersion(t *testing.T) {
	first, err := DeliverableSubmitted("c@example.fr", "Inès", "Refonte <Iris>", "Maquettes", 1, "https://p.test/client/livrables/x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.Subject, "Nouveau livrable") || strings.Contains(first.Text, "version 1") {
		t.Errorf("premiere version : %q / %q", first.Subject, first.Text)
	}
	if strings.Contains(first.HTML, "<Iris>") {
		t.Error("le nom du projet n'est pas echappe")
	}

	second, err := DeliverableSubmitted("c@example.fr", "", "Refonte", "Maquettes", 2, "https://p.test/client/livrables/x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second.Subject, "Nouvelle version") || !strings.Contains(second.Text, "version 2") ||
		!strings.Contains(second.Text, "https://p.test/client/livrables/x") {
		t.Errorf("seconde version : %q / %q", second.Subject, second.Text)
	}
}
