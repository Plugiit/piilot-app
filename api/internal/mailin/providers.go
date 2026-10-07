package mailin

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/mail"
	"strings"
)

// Les webhooks des fournisseurs d'e-mail. Chacun decrit l'e-mail a sa facon ;
// tous finissent en message MIME, deposes en base tels quels.
//
//   - postmark : JSON « Inbound », pieces jointes comprises. Le message brut
//     (RawEmail) est pris tel quel quand l'option est cochee chez Postmark.
//   - mailgun : formulaire d'une route. Une route vers une adresse finissant
//     par /mime envoie le message brut (body-mime) ; sinon, les champs.
//   - brevo : JSON « Inbound parsing ». Brevo ne joint pas les pieces jointes
//     au webhook — il faudrait les telecharger par son API, un appel sortant
//     que la requete ne fait pas : elles sont signalees dans le texte.
//   - raw : le corps de la requete est l'e-mail MIME lui-meme (Cloudflare
//     Email Workers, CloudMailin en mode brut, un script maison).

// Providers liste les fournisseurs reconnus.
var Providers = []string{"postmark", "mailgun", "brevo", "raw"}

// ErrUnknownProvider : fournisseur inconnu.
var ErrUnknownProvider = errors.New("fournisseur inconnu")

// FromWebhook rend le ou les e-mails MIME portes par un webhook.
//
// form est le formulaire multipart deja lu pour Mailgun, nul sinon.
func FromWebhook(provider string, body []byte, form *multipart.Form) ([][]byte, error) {
	switch provider {
	case "raw":
		if len(body) == 0 {
			return nil, errors.New("corps vide")
		}
		return [][]byte{body}, nil
	case "postmark":
		raw, err := fromPostmark(body)
		if err != nil {
			return nil, err
		}
		return [][]byte{raw}, nil
	case "mailgun":
		raw, err := fromMailgun(form)
		if err != nil {
			return nil, err
		}
		return [][]byte{raw}, nil
	case "brevo":
		return fromBrevo(body)
	default:
		return nil, ErrUnknownProvider
	}
}

type postmarkAddress struct {
	Email string `json:"Email"`
	Name  string `json:"Name"`
}

func fromPostmark(body []byte) ([]byte, error) {
	var p struct {
		MessageID         string            `json:"MessageID"`
		FromFull          postmarkAddress   `json:"FromFull"`
		ToFull            []postmarkAddress `json:"ToFull"`
		CcFull            []postmarkAddress `json:"CcFull"`
		OriginalRecipient string            `json:"OriginalRecipient"`
		Subject           string            `json:"Subject"`
		TextBody          string            `json:"TextBody"`
		HTMLBody          string            `json:"HtmlBody"`
		RawEmail          string            `json:"RawEmail"`
		Headers           []struct {
			Name  string `json:"Name"`
			Value string `json:"Value"`
		} `json:"Headers"`
		Attachments []struct {
			Name        string `json:"Name"`
			Content     string `json:"Content"`
			ContentType string `json:"ContentType"`
		} `json:"Attachments"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("JSON Postmark illisible : %w", err)
	}
	if strings.TrimSpace(p.RawEmail) != "" {
		return []byte(p.RawEmail), nil
	}

	d := Draft{
		From:     Address{Name: p.FromFull.Name, Address: p.FromFull.Email},
		Envelope: p.OriginalRecipient,
		Subject:  p.Subject,
		Text:     p.TextBody,
		HTML:     p.HTMLBody,
		Headers:  map[string]string{},
	}
	for _, a := range p.ToFull {
		d.To = append(d.To, Address{Name: a.Name, Address: a.Email})
	}
	for _, a := range p.CcFull {
		d.Cc = append(d.Cc, Address{Name: a.Name, Address: a.Email})
	}
	for _, h := range p.Headers {
		switch strings.ToLower(h.Name) {
		case "message-id":
			d.MessageID = h.Value
		case "in-reply-to":
			d.InReplyTo = h.Value
		case "references":
			d.References = h.Value
		default:
			d.Headers[h.Name] = h.Value
		}
	}
	// Postmark donne son propre identifiant ; l'en-tete Message-ID, quand il
	// est la, est celui de l'expediteur et sert au dedoublonnage.
	if d.MessageID == "" && p.MessageID != "" {
		d.MessageID = p.MessageID + "@postmark"
	}
	for _, a := range p.Attachments {
		content, err := base64.StdEncoding.DecodeString(a.Content)
		if err != nil {
			continue
		}
		d.Attachments = append(d.Attachments, Attachment{Filename: a.Name, ContentType: a.ContentType, Content: content})
	}
	return Compose(d)
}

func fromMailgun(form *multipart.Form) ([]byte, error) {
	if form == nil {
		return nil, errors.New("formulaire Mailgun attendu")
	}
	get := func(k string) string {
		if v := form.Value[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	if raw := get("body-mime"); raw != "" {
		return []byte(raw), nil
	}

	d := Draft{
		MessageID:  get("Message-Id"),
		InReplyTo:  get("In-Reply-To"),
		References: get("References"),
		Envelope:   get("recipient"),
		Subject:    get("subject"),
		Text:       get("body-plain"),
		HTML:       get("body-html"),
		Headers:    map[string]string{},
	}
	if from, err := mail.ParseAddress(get("from")); err == nil {
		d.From = Address{Name: from.Name, Address: from.Address}
	} else {
		d.From = Address{Address: get("sender")}
	}
	if list, err := mail.ParseAddressList(get("To")); err == nil {
		for _, a := range list {
			d.To = append(d.To, Address{Name: a.Name, Address: a.Address})
		}
	}
	// message-headers : la liste complete, en JSON [[nom, valeur], …].
	var headers [][2]string
	if err := json.Unmarshal([]byte(get("message-headers")), &headers); err == nil {
		for _, h := range headers {
			d.Headers[h[0]] = h[1]
			switch strings.ToLower(h[0]) {
			case "message-id":
				if d.MessageID == "" {
					d.MessageID = h[1]
				}
			case "in-reply-to":
				if d.InReplyTo == "" {
					d.InReplyTo = h[1]
				}
			case "references":
				if d.References == "" {
					d.References = h[1]
				}
			}
		}
	}
	for _, files := range form.File {
		for _, fh := range files {
			f, err := fh.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(io.LimitReader(f, MaxRawBytes))
			f.Close()
			if err != nil {
				continue
			}
			d.Attachments = append(d.Attachments, Attachment{
				Filename: fh.Filename, ContentType: fh.Header.Get("Content-Type"), Content: content,
			})
		}
	}
	return Compose(d)
}

type brevoAddress struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

func fromBrevo(body []byte) ([][]byte, error) {
	var p struct {
		Items []struct {
			MessageID   string            `json:"MessageId"`
			InReplyTo   string            `json:"InReplyTo"`
			From        brevoAddress      `json:"From"`
			To          []brevoAddress    `json:"To"`
			Cc          []brevoAddress    `json:"Cc"`
			Recipients  []string          `json:"Recipients"`
			Subject     string            `json:"Subject"`
			RawTextBody string            `json:"RawTextBody"`
			RawHTMLBody string            `json:"RawHtmlBody"`
			Headers     map[string]any    `json:"Headers"`
			Attachments []json.RawMessage `json:"Attachments"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("JSON Brevo illisible : %w", err)
	}

	out := make([][]byte, 0, len(p.Items))
	for _, it := range p.Items {
		d := Draft{
			MessageID: it.MessageID,
			InReplyTo: it.InReplyTo,
			From:      Address{Name: it.From.Name, Address: it.From.Address},
			Subject:   it.Subject,
			Text:      it.RawTextBody,
			HTML:      it.RawHTMLBody,
			Headers:   map[string]string{},
		}
		if len(it.Recipients) > 0 {
			d.Envelope = it.Recipients[0]
		}
		for _, a := range it.To {
			d.To = append(d.To, Address{Name: a.Name, Address: a.Address})
		}
		for _, a := range it.Cc {
			d.Cc = append(d.Cc, Address{Name: a.Name, Address: a.Address})
		}
		for k, v := range it.Headers {
			if s, ok := v.(string); ok {
				d.Headers[k] = s
				if strings.EqualFold(k, "references") {
					d.References = s
				}
			}
		}
		if n := len(it.Attachments); n > 0 {
			note := fmt.Sprintf("\n\n[%d pièce(s) jointe(s) non récupérée(s) : Brevo ne les transmet pas au webhook.]", n)
			d.Text += note
		}
		raw, err := Compose(d)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	if len(out) == 0 {
		return nil, errors.New("aucun e-mail dans le webhook Brevo")
	}
	return out, nil
}
