package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// Securite de la connexion au serveur SMTP.
const (
	// SecurityStartTLS : connexion en clair puis passage en TLS (port 587).
	// Le defaut, et le passage en TLS est exige : un serveur qui ne le propose
	// pas est refuse plutot que de faire circuler les liens en clair.
	SecurityStartTLS = "starttls"
	// SecurityTLS : TLS des la connexion (port 465).
	SecurityTLS = "tls"
	// SecurityNone : aucun chiffrement. Pour un serveur local de test
	// (Mailpit) seulement.
	SecurityNone = "none"
)

// SMTP envoie par un serveur SMTP.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	// From est l'expediteur, « Piilot <piilot@agence.fr> » ou une adresse
	// nue.
	From     string
	Security string
}

// Configured dit si un serveur est renseigne. Sans lui, Piilot n'envoie aucun
// e-mail : les liens d'invitation et de reinitialisation se copient depuis
// l'ecran des comptes.
func (s SMTP) Configured() bool {
	return s.Host != "" && s.From != ""
}

// Validate verifie la configuration au demarrage.
func (s SMTP) Validate() error {
	if !s.Configured() {
		return nil
	}
	if _, err := mail.ParseAddress(s.From); err != nil {
		return fmt.Errorf("SMTP_FROM invalide (%q) : %w", s.From, err)
	}
	switch s.Security {
	case SecurityStartTLS, SecurityTLS, SecurityNone:
	default:
		return fmt.Errorf("SMTP_SECURITY invalide (%q) : starttls, tls ou none", s.Security)
	}
	return nil
}

// Send envoie un message. Le contexte borne la duree de toute l'operation.
func (s SMTP) Send(ctx context.Context, msg Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("expediteur invalide : %w", err)
	}
	if _, err := mail.ParseAddress(msg.To); err != nil {
		return fmt.Errorf("destinataire invalide : %w", err)
	}

	raw, err := build(from, msg)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(time.Minute)
	}
	dialer := &net.Dialer{Deadline: deadline}

	var conn net.Conn
	if s.Security == SecurityTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connexion a %s : %w", addr, err)
	}
	_ = conn.SetDeadline(deadline)

	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("dialogue SMTP : %w", err)
	}
	defer client.Close()

	if s.Security == SecurityStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("le serveur SMTP ne propose pas STARTTLS : SMTP_SECURITY=tls pour le port 465")
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("passage en TLS : %w", err)
		}
	}

	if s.Username != "" {
		// PlainAuth refuse d'envoyer le mot de passe sur une connexion non
		// chiffree, sauf vers localhost : c'est le comportement voulu.
		if err := client.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return fmt.Errorf("authentification SMTP : %w", err)
		}
	}

	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("expediteur refuse : %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("destinataire refuse : %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("envoi du message : %w", err)
	}
	if _, err := w.Write(raw); err != nil {
		return fmt.Errorf("envoi du message : %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("envoi du message : %w", err)
	}

	return client.Quit()
}

// build assemble le message MIME : une version texte et une version HTML,
// que le client de messagerie choisit selon ce qu'il sait afficher.
func build(from *mail.Address, msg Message) ([]byte, error) {
	var body bytes.Buffer
	parts := multipart.NewWriter(&body)

	for _, part := range []struct{ contentType, content string }{
		{"text/plain; charset=utf-8", msg.Text},
		{"text/html; charset=utf-8", msg.HTML},
	} {
		w, err := parts.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.contentType},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return nil, fmt.Errorf("composition MIME : %w", err)
		}
		if _, err := w.Write(wrap76(base64.StdEncoding.EncodeToString([]byte(part.content)))); err != nil {
			return nil, fmt.Errorf("composition MIME : %w", err)
		}
	}
	if err := parts.Close(); err != nil {
		return nil, fmt.Errorf("composition MIME : %w", err)
	}

	id := make([]byte, 16)
	_, _ = rand.Read(id)
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]
	messageID := "<" + hex.EncodeToString(id) + "@" + domain + ">"
	if msg.MessageID != "" {
		messageID = "<" + strings.Trim(msg.MessageID, "<>") + ">"
	}

	var out bytes.Buffer
	headers := [][2]string{
		{"From", from.String()},
		{"To", msg.To},
		{"Subject", mime.QEncoding.Encode("utf-8", msg.Subject)},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", messageID},
		{"MIME-Version", "1.0"},
		{"Content-Type", "multipart/alternative; boundary=" + parts.Boundary()},
		// Les e-mails de Piilot sont des notifications automatiques : les
		// repondeurs d'absence n'ont pas a y repondre.
		{"Auto-Submitted", "auto-generated"},
	}
	if msg.ReplyTo != "" {
		headers = append(headers, [2]string{"Reply-To", msg.ReplyTo})
	}
	if msg.InReplyTo != "" {
		ref := "<" + strings.Trim(msg.InReplyTo, "<>") + ">"
		headers = append(headers, [2]string{"In-Reply-To", ref}, [2]string{"References", ref})
	}
	for _, h := range headers {
		fmt.Fprintf(&out, "%s: %s\r\n", h[0], h[1])
	}
	out.WriteString("\r\n")
	out.Write(body.Bytes())

	return out.Bytes(), nil
}

// wrap76 coupe le base64 en lignes de 76 caracteres, la longueur maximale
// qu'impose la norme MIME.
func wrap76(s string) []byte {
	var out bytes.Buffer
	for len(s) > 76 {
		out.WriteString(s[:76])
		out.WriteString("\r\n")
		s = s[76:]
	}
	out.WriteString(s)
	out.WriteString("\r\n")

	return out.Bytes()
}
