package mailin

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
)

const gmailReply = "From: =?UTF-8?Q?In=C3=A8s_Client?= <ines@client.fr>\r\n" +
	"To: support+t47.abcdef0123@agence.fr\r\n" +
	"Subject: Re: =?UTF-8?Q?R=C3=A9ponse_sur_votre_demande_#47?=\r\n" +
	"Message-ID: <CAF123@mail.gmail.com>\r\n" +
	"In-Reply-To: <t47.abcdef0123.9f8e@agence.fr>\r\n" +
	"References: <t47.abcdef0123.0000@agence.fr> <t47.abcdef0123.9f8e@agence.fr>\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: multipart/mixed; boundary=\"mixed\"\r\n" +
	"\r\n" +
	"--mixed\r\n" +
	"Content-Type: multipart/alternative; boundary=\"alt\"\r\n" +
	"\r\n" +
	"--alt\r\n" +
	"Content-Type: text/plain; charset=\"UTF-8\"\r\n" +
	"Content-Transfer-Encoding: quoted-printable\r\n" +
	"\r\n" +
	"Merci, c'est r=C3=A9gl=C3=A9 de mon c=C3=B4t=C3=A9.\r\n" +
	"\r\n" +
	"Le lun. 7 oct. 2026 =C3=A0 10:02, Piilot <support@agence.fr> a\r\n" +
	"=C3=A9crit :\r\n" +
	"\r\n" +
	"> L'agence a r=C3=A9pondu =C3=A0 votre demande.\r\n" +
	"--alt\r\n" +
	"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
	"\r\n" +
	"<div>Merci</div><div class=\"gmail_quote\">cit\xc3\xa9</div>\r\n" +
	"--alt--\r\n" +
	"--mixed\r\n" +
	"Content-Type: image/png; name=\"capture.png\"\r\n" +
	"Content-Disposition: attachment; filename=\"capture.png\"\r\n" +
	"Content-Transfer-Encoding: base64\r\n" +
	"\r\n" +
	"iVBORw0KGgo=\r\n" +
	"--mixed\r\n" +
	"Content-Type: image/png\r\n" +
	"Content-ID: <logo@sig>\r\n" +
	"Content-Disposition: inline\r\n" +
	"Content-Transfer-Encoding: base64\r\n" +
	"\r\n" +
	"iVBORw0KGgo=\r\n" +
	"--mixed--\r\n"

func TestParseReponseGmail(t *testing.T) {
	m, err := Parse([]byte(gmailReply))
	if err != nil {
		t.Fatal(err)
	}
	if m.From.Address != "ines@client.fr" || m.From.Name != "Inès Client" {
		t.Errorf("expediteur : %+v", m.From)
	}
	if m.MessageID != "CAF123@mail.gmail.com" || len(m.References) != 2 || m.InReplyTo[0] != "t47.abcdef0123.9f8e@agence.fr" {
		t.Errorf("fil : %q %v %v", m.MessageID, m.InReplyTo, m.References)
	}
	if m.Subject != "Re: Réponse sur votre demande #47" || CleanSubject(m.Subject) != "Réponse sur votre demande #47" {
		t.Errorf("objet : %q", m.Subject)
	}
	if got := m.Body(); got != "Merci, c'est réglé de mon côté." {
		t.Errorf("corps : %q", got)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Filename != "capture.png" {
		t.Errorf("pieces jointes (le logo de signature doit etre ecarte) : %+v", m.Attachments)
	}
	if m.Automatic {
		t.Error("une reponse ecrite a la main passe pour automatique")
	}
}

func TestParseLatin1EtHTMLSeul(t *testing.T) {
	raw := "From: a@b.fr\r\nSubject: =?ISO-8859-1?Q?Probl=E8me?=\r\n" +
		"Content-Type: text/html; charset=windows-1252\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"<html><head><style>p{}</style></head><body><p>Le formulaire est cass=E9 =80 cause du bouton.</p>" +
		"<div id=3D\"divRplyFwdMsg\">De : Piilot</div></body></html>\r\n"
	m, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject != "Problème" {
		t.Errorf("objet : %q", m.Subject)
	}
	if got := m.Body(); got != "Le formulaire est cassé € cause du bouton." {
		t.Errorf("corps : %q", got)
	}
}

func TestAutomatiques(t *testing.T) {
	for _, raw := range []string{
		"From: a@b.fr\r\nAuto-Submitted: auto-replied\r\nSubject: Absent\r\n\r\nJe suis absent.",
		"From: MAILER-DAEMON@b.fr\r\nSubject: Undelivered\r\n\r\nx",
		"From: a@b.fr\r\nPrecedence: bulk\r\nSubject: x\r\n\r\nx",
		"From: a@b.fr\r\nContent-Type: multipart/report; boundary=r\r\n\r\n--r\r\nContent-Type: text/plain\r\n\r\nx\r\n--r--\r\n",
	} {
		m, err := Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if !m.Automatic {
			t.Errorf("non reconnu comme automatique : %q", raw[:40])
		}
	}
}

func TestStripReply(t *testing.T) {
	cases := map[string]string{
		"Outlook FR":         "C'est bon pour moi.\n\nCordialement,\nInès\n\n________________________________\nDe : Piilot <support@agence.fr>\nEnvoyé : lundi 7 octobre 2026 10:02\nÀ : Inès\nObjet : Réponse",
		"Outlook sans trait": "C'est bon pour moi.\n\nCordialement,\nInès\n\nDe : Piilot <support@agence.fr>\nEnvoyé : lundi 7 octobre 2026 10:02\nObjet : Réponse",
		"Anglais":            "C'est bon pour moi.\n\nCordialement,\nInès\n\nOn Mon, Oct 7, 2026 at 10:02 AM Piilot <support@agence.fr> wrote:\n> Hello",
		"Signature":          "C'est bon pour moi.\n\nCordialement,\nInès\n-- \nInès Client\n06 00 00 00 00",
		"iPhone":             "C'est bon pour moi.\n\nCordialement,\nInès\n\nEnvoyé de mon iPhone\n\n> Le 7 oct. 2026 à 10:02, Piilot a écrit :",
		"Message origine":    "C'est bon pour moi.\n\nCordialement,\nInès\n\n-----Message d'origine-----\nDe: x",
	}
	for name, in := range cases {
		if got := StripReply(in); got != "C'est bon pour moi.\n\nCordialement,\nInès" {
			t.Errorf("%s : %q", name, got)
		}
	}
	// Une citation au milieu d'une reponse reste : on y repond.
	inline := "> Le bouton ne marche pas ?\nSi, il marche maintenant.\n\nMerci"
	if got := StripReply(inline); got != inline {
		t.Errorf("citation au milieu : %q", got)
	}
	// Tout coupe : on rend tout.
	if got := StripReply("> seulement une citation"); got != "> seulement une citation" {
		t.Errorf("tout coupe : %q", got)
	}
}

func TestPostmarkMailgunBrevo(t *testing.T) {
	pm, _ := json.Marshal(map[string]any{
		"MessageID": "pm-1",
		"FromFull":  map[string]string{"Email": "ines@client.fr", "Name": "Inès"},
		"ToFull":    []map[string]string{{"Email": "support+t47.abcdef0123@agence.fr"}},
		"Subject":   "Re: #47",
		"TextBody":  "Réglé, merci.\n\nLe lun. 7 oct. 2026 à 10:02, Piilot a écrit :\n> x",
		"Headers":   []map[string]string{{"Name": "Message-ID", "Value": "<abc@client.fr>"}, {"Name": "In-Reply-To", "Value": "<t47.abcdef0123.1@agence.fr>"}},
		"Attachments": []map[string]string{{"Name": "devis.pdf", "ContentType": "application/pdf",
			"Content": base64.StdEncoding.EncodeToString([]byte("%PDF-1.4"))}},
	})
	raws, err := FromWebhook("postmark", pm, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(raws[0])
	if err != nil {
		t.Fatal(err)
	}
	if m.MessageID != "abc@client.fr" || m.InReplyTo[0] != "t47.abcdef0123.1@agence.fr" || m.Body() != "Réglé, merci." ||
		len(m.Attachments) != 1 || string(m.Attachments[0].Content) != "%PDF-1.4" || m.To[0].Address != "support+t47.abcdef0123@agence.fr" {
		t.Errorf("postmark : %+v", m)
	}

	brevo, _ := json.Marshal(map[string]any{"items": []map[string]any{{
		"MessageId": "<b-1@client.fr>", "From": map[string]string{"Name": "Inès", "Address": "ines@client.fr"},
		"Recipients": []string{"support@agence.fr"}, "Subject": "Nouveau", "RawTextBody": "Le site est lent.",
		"Attachments": []map[string]string{{"Name": "x.png"}},
	}}})
	raws, err = FromWebhook("brevo", brevo, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, _ = Parse(raws[0])
	if m.MessageID != "b-1@client.fr" || !strings.Contains(m.Text, "non récupérée") || m.Envelope[0] != "support@agence.fr" {
		t.Errorf("brevo : %+v", m)
	}

	if _, err := FromWebhook("inconnu", nil, nil); err != ErrUnknownProvider {
		t.Errorf("fournisseur inconnu : %v", err)
	}
}

// Un faux serveur IMAP, juste assez bavard pour la releve.
func TestIMAP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	message := "From: ines@client.fr\r\nSubject: Bonjour\r\n\r\nUn corps.\r\n"
	seen := make(chan string, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		fmt.Fprint(conn, "* OK IMAP pret\r\n")
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return
			}
			tag, cmd, _ := strings.Cut(strings.TrimSpace(l), " ")
			switch {
			case strings.HasPrefix(cmd, "LOGIN"):
				// Le mot de passe accentue arrive en litteral.
				if strings.HasSuffix(cmd, "}") {
					fmt.Fprint(conn, "+ vas-y\r\n")
					rest, _ := r.ReadString('\n')
					if !strings.Contains(rest, "mot-de-passé") {
						fmt.Fprintf(conn, "%s NO mauvais mot de passe\r\n", tag)
						continue
					}
				}
				fmt.Fprintf(conn, "%s OK connecte\r\n", tag)
			case strings.HasPrefix(cmd, "SELECT"):
				fmt.Fprintf(conn, "* 2 EXISTS\r\n%s OK [READ-WRITE] ouvert\r\n", tag)
			case cmd == "UID SEARCH UNSEEN":
				fmt.Fprintf(conn, "* SEARCH 7 9\r\n%s OK fait\r\n", tag)
			case strings.HasPrefix(cmd, "UID FETCH 9"):
				fmt.Fprintf(conn, "* 2 FETCH (UID 9 BODY[] {%d}\r\n%s)\r\n%s OK fait\r\n", len(message), message, tag)
			case strings.HasPrefix(cmd, "UID STORE"):
				seen <- cmd
				fmt.Fprintf(conn, "%s OK fait\r\n", tag)
			case cmd == "LOGOUT":
				fmt.Fprintf(conn, "* BYE\r\n%s OK au revoir\r\n", tag)
				return
			default:
				fmt.Fprintf(conn, "%s BAD inconnu\r\n", tag)
			}
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	c, err := DialIMAP(IMAPConfig{Host: "127.0.0.1", Port: addr.Port, Username: "support", Password: "mot-de-passé"})
	if err != nil {
		t.Fatal(err)
	}
	uids, err := c.Unseen()
	if err != nil || len(uids) != 2 || uids[1] != 9 {
		t.Fatalf("non lus : %v %v", uids, err)
	}
	raw, err := c.Fetch(9)
	if err != nil || string(raw) != message {
		t.Fatalf("lecture : %q %v", raw, err)
	}
	if err := c.MarkSeen(9); err != nil {
		t.Fatal(err)
	}
	if got := <-seen; !strings.Contains(got, `\Seen`) {
		t.Errorf("marquage : %q", got)
	}
	_ = c.Close()
}
