package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"
)

// Codes de liaison : tapes a la main dans Discord, donc courts et sans
// caractere ambigu.
//
// Base32 de Crockford : l'alphabet exclut deja I, L, O et U. A la lecture, O
// vaut 0, I et L valent 1 — une confusion de saisie ne fait donc jamais
// echouer un code. Insensible a la casse et au tiret.
//
// 8 caracteres, soit 40 bits. Peu face a un hachage hors ligne, d'ou le HMAC
// sous une cle derivee du secret applicatif ; assez face a des essais en
// ligne, que la limitation de debit borne et que l'expiration rend vains.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var linkCodeEncoding = base32.NewEncoding(crockford).WithPadding(base32.NoPadding)

const (
	linkCodeBytes = 5
	linkCodeLen   = 8
)

// NewLinkCode tire un code, sous sa forme normalisee (8 caracteres, sans tiret).
func NewLinkCode() (string, error) {
	buf := make([]byte, linkCodeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generation du code de liaison : %w", err)
	}
	return linkCodeEncoding.EncodeToString(buf), nil
}

// FormatLinkCode presente un code normalise en deux groupes : XXXX-XXXX.
func FormatLinkCode(code string) string {
	if len(code) != linkCodeLen {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// NormalizeLinkCode ramene une saisie a la forme normalisee, ou faux si elle
// ne peut pas etre un code.
func NormalizeLinkCode(input string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(input) {
		switch r {
		case '-', ' ':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		if !strings.ContainsRune(crockford, r) {
			return "", false
		}
		b.WriteRune(r)
	}
	if b.Len() != linkCodeLen {
		return "", false
	}
	return b.String(), true
}

// HashLinkCode rend l'empreinte stockee d'un code normalise.
func HashLinkCode(key []byte, code string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(code))
	return mac.Sum(nil)
}
