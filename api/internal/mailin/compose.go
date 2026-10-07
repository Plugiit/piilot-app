package mailin

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

// Draft est un e-mail tel qu'un fournisseur le decrit en JSON. Compose en fait
// un message MIME : toutes les portes d'entree — IMAP, webhooks — deposent la
// meme chose en base, et un seul lecteur les traite.
type Draft struct {
	MessageID string
	InReplyTo string
	// References, separees par des espaces, chevrons compris ou non.
	References string
	From       Address
	To         []Address
	Cc         []Address
	// Adresse a laquelle le fournisseur a remis l'e-mail : l'adresse plus,
	// quand elle n'apparait ni dans To ni dans Cc.
	Envelope    string
	Subject     string
	Date        time.Time
	Text        string
	HTML        string
	Headers     map[string]string
	Attachments []Attachment
}

// Compose assemble le message MIME d'un Draft.
func Compose(d Draft) ([]byte, error) {
	var body bytes.Buffer
	mixed := multipart.NewWriter(&body)

	var altBody bytes.Buffer
	alt := multipart.NewWriter(&altBody)
	for _, part := range []struct{ ct, content string }{
		{"text/plain; charset=utf-8", d.Text},
		{"text/html; charset=utf-8", d.HTML},
	} {
		if part.content == "" {
			continue
		}
		w, err := alt.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ct},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(base64Lines([]byte(part.content))); err != nil {
			return nil, err
		}
	}
	if err := alt.Close(); err != nil {
		return nil, err
	}

	w, err := mixed.CreatePart(textproto.MIMEHeader{
		"Content-Type": {"multipart/alternative; boundary=" + alt.Boundary()},
	})
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(altBody.Bytes()); err != nil {
		return nil, err
	}

	for _, a := range d.Attachments {
		ct := a.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		w, err := mixed.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {mime.FormatMediaType(ct, map[string]string{"name": a.Filename})},
			"Content-Disposition":       {mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename})},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(base64Lines(a.Content)); err != nil {
			return nil, err
		}
	}
	if err := mixed.Close(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	write := func(k, v string) {
		v = strings.NewReplacer("\r", " ", "\n", " ").Replace(v)
		if strings.TrimSpace(v) != "" {
			fmt.Fprintf(&out, "%s: %s\r\n", k, v)
		}
	}
	write("From", formatAddress(d.From))
	write("To", formatList(d.To))
	write("Cc", formatList(d.Cc))
	if d.Envelope != "" {
		write("Delivered-To", d.Envelope)
	}
	write("Subject", mime.QEncoding.Encode("utf-8", d.Subject))
	date := d.Date
	if date.IsZero() {
		date = time.Now()
	}
	write("Date", date.Format(time.RFC1123Z))
	write("Message-ID", bracket(d.MessageID))
	write("In-Reply-To", bracketList(d.InReplyTo))
	write("References", bracketList(d.References))
	for k, v := range d.Headers {
		switch textproto.CanonicalMIMEHeaderKey(k) {
		case "Auto-Submitted", "Precedence", "X-Autoreply", "X-Autorespond", "List-Id", "List-Unsubscribe":
			write(textproto.CanonicalMIMEHeaderKey(k), v)
		}
	}
	write("MIME-Version", "1.0")
	write("Content-Type", "multipart/mixed; boundary="+mixed.Boundary())
	out.WriteString("\r\n")
	out.Write(body.Bytes())

	return out.Bytes(), nil
}

func formatAddress(a Address) string {
	if a.Address == "" {
		return ""
	}
	return (&mail.Address{Name: a.Name, Address: a.Address}).String()
}

func formatList(list []Address) string {
	parts := make([]string, 0, len(list))
	for _, a := range list {
		if s := formatAddress(a); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

func bracket(id string) string {
	id = strings.Trim(strings.TrimSpace(id), "<>")
	if id == "" {
		return ""
	}
	return "<" + id + ">"
}

func bracketList(v string) string {
	fields := strings.Fields(strings.NewReplacer(",", " ").Replace(v))
	for i, f := range fields {
		fields[i] = bracket(f)
	}
	return strings.Join(fields, " ")
}

func base64Lines(b []byte) []byte {
	enc := base64.StdEncoding.EncodeToString(b)
	var out bytes.Buffer
	for len(enc) > 76 {
		out.WriteString(enc[:76])
		out.WriteString("\r\n")
		enc = enc[76:]
	}
	out.WriteString(enc)
	out.WriteString("\r\n")
	return out.Bytes()
}
