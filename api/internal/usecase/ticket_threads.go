package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/plugiit/piilot-app/api/internal/repository/db"
	"github.com/plugiit/piilot-app/api/internal/security"
)

// Le fil e-mail d'un ticket.
//
// Chaque e-mail d'un ticket porte une etiquette « t47.<signature> » : dans
// son adresse de reponse (support+t47.3fa9c1d0be@agence.fr) et dans son
// Message-ID. La reponse du client la rapporte d'une facon ou de l'autre —
// l'adresse quand il repond, l'en-tete In-Reply-To toujours — et l'e-mail
// rejoint le bon ticket.
//
// La signature (HMAC tronque a 40 bits, cle derivee de JWT_SECRET) empeche
// d'ecrire sur un ticket en devinant son numero : support+t48@ ne mene nulle
// part sans la signature du 48.

const ticketTagPurpose = "ticket-reply-tag"

var ticketTagPattern = regexp.MustCompile(`(?i)\bt(\d{1,12})\.([0-9a-f]{10})\b`)

// threading porte ce qu'il faut pour filer les e-mails des tickets.
type threading struct {
	key []byte
	// Adresse de support venue de l'environnement ; vide, celle de l'ecran.
	envAddress string
}

func newThreading(secret []byte, envAddress string) threading {
	if len(secret) == 0 {
		return threading{envAddress: envAddress}
	}
	return threading{key: security.DeriveKey(secret, ticketTagPurpose), envAddress: strings.TrimSpace(envAddress)}
}

func (t threading) tag(numero int64) string {
	mac := hmac.New(sha256.New, t.key)
	fmt.Fprintf(mac, "t%d", numero)
	return fmt.Sprintf("t%d.%s", numero, hex.EncodeToString(mac.Sum(nil))[:10])
}

// numeros rend les numeros de ticket dont une etiquette valide apparait dans
// ces textes : adresses, Message-ID, References.
func (t threading) numeros(texts ...string) []int64 {
	if len(t.key) == 0 {
		return nil
	}
	seen := map[int64]bool{}
	var out []int64
	for _, text := range texts {
		for _, m := range ticketTagPattern.FindAllStringSubmatch(text, -1) {
			n, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil || seen[n] {
				continue
			}
			if hmac.Equal([]byte(strings.ToLower(t.tag(n))), []byte(strings.ToLower(m[0]))) {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// address rend l'adresse de support en vigueur : l'environnement prime.
func (t threading) address(ctx context.Context, q *db.Queries) string {
	if t.envAddress != "" {
		return t.envAddress
	}
	settings, err := q.GetInboundSettings(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(settings.Address)
}

// headers rend l'adresse de reponse, l'identifiant du message et celui du
// fil d'un e-mail de ticket. Vides quand l'e-mail entrant n'est pas regle :
// l'e-mail part alors comme avant, sans invitation a repondre.
func (t threading) headers(ctx context.Context, q *db.Queries, numero int64) (replyTo, messageID, inReplyTo string) {
	if len(t.key) == 0 {
		return "", "", ""
	}
	addr := t.address(ctx, q)
	local, domain, ok := strings.Cut(addr, "@")
	if !ok || local == "" || domain == "" {
		return "", "", ""
	}
	if plus := strings.IndexByte(local, '+'); plus >= 0 {
		local = local[:plus]
	}
	tag := t.tag(numero)
	random := make([]byte, 6)
	_, _ = rand.Read(random)
	return fmt.Sprintf("%s+%s@%s", local, tag, domain),
		fmt.Sprintf("%s.%s@%s", tag, hex.EncodeToString(random), domain),
		fmt.Sprintf("%s@%s", tag, domain)
}

// nullableText rend nul un texte vide, pour une colonne qui distingue
// « rien » de « vide ».
func nullableText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
