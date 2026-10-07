// Package mailin lit les e-mails entrants : le format MIME, les citations a
// couper, les formats des fournisseurs, et une boite IMAP.
//
// Tout tient dans la bibliotheque standard. Les clients IMAP et les lecteurs
// MIME du commerce couvrent cent fois ce qu'il faut ici — relever une boite,
// lire un texte et des pieces jointes — pour autant de code a suivre.
package mailin

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Limites d'un e-mail entrant. Au-dela, il est refuse plutot que lu a moitie.
const (
	MaxRawBytes    = 30 << 20
	MaxAttachments = 10
	maxDepth       = 8
)

// Address est une adresse lue dans un en-tete.
type Address struct {
	Name    string
	Address string
}

// Attachment est une piece jointe.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// Message est un e-mail entrant, lu.
type Message struct {
	MessageID  string
	InReplyTo  []string
	References []string
	From       Address
	To         []Address
	Cc         []Address
	// Destinataires d'enveloppe que certains serveurs ajoutent :
	// Delivered-To, X-Original-To. Ils portent l'adresse plus quand le
	// client a repondu a tous.
	Envelope []string
	Subject  string
	Date     time.Time
	Text     string
	HTML     string
	// Reponse automatique, rebond, liste de diffusion : rien a ranger.
	Automatic   bool
	Attachments []Attachment
}

// ErrTooLarge : l'e-mail depasse MaxRawBytes.
var ErrTooLarge = errors.New("e-mail trop volumineux")

var wordDecoder = &mime.WordDecoder{CharsetReader: charsetReader}

// Parse lit un e-mail au format MIME.
func Parse(raw []byte) (Message, error) {
	if len(raw) > MaxRawBytes {
		return Message{}, ErrTooLarge
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Message{}, fmt.Errorf("en-tetes illisibles : %w", err)
	}
	h := msg.Header

	out := Message{
		MessageID:  firstID(h.Get("Message-Id")),
		InReplyTo:  messageIDs(h.Get("In-Reply-To")),
		References: messageIDs(h.Get("References")),
		Subject:    decodeHeader(h.Get("Subject")),
	}
	if from := addresses(h, "From"); len(from) > 0 {
		out.From = from[0]
	}
	out.To = addresses(h, "To")
	out.Cc = addresses(h, "Cc")
	for _, key := range []string{"Delivered-To", "X-Original-To", "Envelope-To"} {
		for _, v := range h[textproto.CanonicalMIMEHeaderKey(key)] {
			if a, err := mail.ParseAddress(strings.TrimSpace(v)); err == nil {
				out.Envelope = append(out.Envelope, strings.ToLower(a.Address))
			} else if v = strings.TrimSpace(v); strings.Contains(v, "@") {
				out.Envelope = append(out.Envelope, strings.ToLower(strings.Trim(v, "<>")))
			}
		}
	}
	if d, err := h.Date(); err == nil {
		out.Date = d
	}
	out.Automatic = isAutomatic(h, out.From.Address)

	if err := walk(&out, textprotoHeader(h), msg.Body, 0); err != nil {
		return Message{}, err
	}

	return out, nil
}

// header est un en-tete de partie : celui du message, ou celui d'une partie
// d'un multipart.
type header interface {
	Get(key string) string
}

type textprotoHeader mail.Header

func (h textprotoHeader) Get(key string) string { return mail.Header(h).Get(key) }

// walk descend dans l'arbre MIME : le premier texte et le premier HTML qui ne
// sont pas des pieces jointes font le corps, le reste est joint.
func walk(out *Message, h header, body io.Reader, depth int) error {
	if depth > maxDepth {
		return nil
	}

	mediaType, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil || mediaType == "" {
		mediaType, params = "text/plain", map[string]string{"charset": "us-ascii"}
	}
	disposition, dparams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))

	if strings.HasPrefix(mediaType, "multipart/") {
		// Un rapport de remise (rebond) : rien a lire pour un ticket.
		if mediaType == "multipart/report" {
			out.Automatic = true
		}
		boundary := params["boundary"]
		if boundary == "" {
			return nil
		}
		mr := multipart.NewReader(body, boundary)
		for {
			part, err := mr.NextRawPart()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				// Un multipart mal ferme : on garde ce qui a ete lu.
				return nil
			}
			if err := walk(out, part.Header, part, depth+1); err != nil {
				return err
			}
		}
	}

	content, err := decodeBody(body, h.Get("Content-Transfer-Encoding"))
	if err != nil {
		return nil
	}

	filename := decodeHeader(dparams["filename"])
	if filename == "" {
		filename = decodeHeader(params["name"])
	}
	attachment := disposition == "attachment" || (filename != "" && !strings.HasPrefix(mediaType, "text/"))

	switch {
	case !attachment && mediaType == "text/plain" && out.Text == "":
		out.Text = toUTF8(content, params["charset"])
	case !attachment && mediaType == "text/html" && out.HTML == "":
		out.HTML = toUTF8(content, params["charset"])
	case mediaType == "message/rfc822":
		if filename == "" {
			filename = "message-transfere.eml"
		}
		out.addAttachment(filename, mediaType, content)
	case attachment || filename != "":
		// Une image en ligne sans nom de fichier est le logo d'une signature :
		// elle n'a pas sa place dans les pieces jointes du ticket.
		if h.Get("Content-Id") != "" && disposition != "attachment" && filename == "" {
			return nil
		}
		if filename == "" {
			filename = "piece-jointe"
		}
		out.addAttachment(filename, mediaType, content)
	}

	return nil
}

func (m *Message) addAttachment(filename, contentType string, content []byte) {
	if len(m.Attachments) >= MaxAttachments || len(content) == 0 {
		return
	}
	name := strings.TrimSpace(filepath.Base(strings.ReplaceAll(filename, `\`, "/")))
	if name == "" || name == "." || name == "/" {
		name = "piece-jointe"
	}
	m.Attachments = append(m.Attachments, Attachment{Filename: name, ContentType: contentType, Content: content})
}

func decodeBody(body io.Reader, encoding string) ([]byte, error) {
	limited := io.LimitReader(body, MaxRawBytes)
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		data, err := io.ReadAll(limited)
		if err != nil {
			return nil, err
		}
		// Des lignes coupees, des espaces : on ne garde que l'alphabet.
		clean := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, data)
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(clean)))
		n, err := base64.StdEncoding.Decode(decoded, clean)
		if err != nil {
			// Remplissage manquant : on tente sans.
			n2, err2 := base64.RawStdEncoding.Decode(decoded, bytes.TrimRight(clean, "="))
			if err2 != nil {
				return decoded[:n], nil
			}
			n = n2
		}
		return decoded[:n], nil
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(limited))
	default:
		return io.ReadAll(limited)
	}
}

func decodeHeader(v string) string {
	if v == "" {
		return ""
	}
	decoded, err := wordDecoder.DecodeHeader(v)
	if err != nil {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(decoded)
}

func addresses(h mail.Header, key string) []Address {
	raw := h.Get(key)
	if raw == "" {
		return nil
	}
	parser := mail.AddressParser{WordDecoder: wordDecoder}
	list, err := parser.ParseList(raw)
	if err != nil {
		// Une liste mal formee : on garde ce qui ressemble a des adresses.
		var out []Address
		for _, m := range looseAddress.FindAllString(raw, -1) {
			out = append(out, Address{Address: strings.ToLower(m)})
		}
		return out
	}
	out := make([]Address, 0, len(list))
	for _, a := range list {
		out = append(out, Address{Name: a.Name, Address: strings.ToLower(a.Address)})
	}
	return out
}

var (
	looseAddress = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	messageID    = regexp.MustCompile(`<([^<>\s]+)>`)
)

func messageIDs(v string) []string {
	var out []string
	for _, m := range messageID.FindAllStringSubmatch(v, -1) {
		out = append(out, m[1])
	}
	return out
}

func firstID(v string) string {
	if ids := messageIDs(v); len(ids) > 0 {
		return ids[0]
	}
	return strings.Trim(strings.TrimSpace(v), "<>")
}

// isAutomatic reconnait ce qu'aucune personne n'a ecrit : absences, rebonds,
// listes. Y repondre par un ticket ferait tourner deux robots l'un contre
// l'autre.
func isAutomatic(h mail.Header, from string) bool {
	if v := strings.ToLower(h.Get("Auto-Submitted")); v != "" && v != "no" {
		return true
	}
	switch strings.ToLower(h.Get("Precedence")) {
	case "bulk", "junk", "list", "auto_reply":
		return true
	}
	for _, key := range []string{"X-Autoreply", "X-Autorespond", "X-Auto-Response-Suppress", "List-Id", "List-Unsubscribe"} {
		if v := strings.ToLower(h.Get(key)); v != "" {
			// Outlook pose X-Auto-Response-Suppress sur des messages ecrits par
			// des personnes : seule la valeur « All » sur une reponse l'est pas.
			if key == "X-Auto-Response-Suppress" {
				continue
			}
			return true
		}
	}
	local := strings.ToLower(from)
	if at := strings.IndexByte(local, '@'); at >= 0 {
		local = local[:at]
	}
	switch local {
	case "mailer-daemon", "postmaster", "noreply", "no-reply", "donotreply", "do-not-reply":
		return true
	}
	return false
}
