package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrBadLink : un lien signe illisible, altere ou perime.
var ErrBadLink = errors.New("lien invalide ou expiré")

// SignLink produit un jeton pour un lien envoye par e-mail : la charge, sa
// date de peremption et une empreinte HMAC. Rien n'est chiffre — la charge
// est un identifiant, pas un secret — mais rien ne se forge sans la cle.
func SignLink(secret []byte, payload string, expires time.Time) string {
	body := payload + "|" + strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString([]byte(body)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyLink relit un jeton de SignLink et rend sa charge.
func VerifyLink(secret []byte, token string, now time.Time) (string, error) {
	encoded, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrBadLink
	}
	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrBadLink
	}
	given, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return "", ErrBadLink
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	if subtle.ConstantTimeCompare(given, mac.Sum(nil)) != 1 {
		return "", ErrBadLink
	}

	// La charge peut elle-meme contenir le separateur : la date est apres le
	// dernier.
	cut := strings.LastIndex(string(body), "|")
	if cut < 0 {
		return "", ErrBadLink
	}
	payload, expiresRaw := string(body[:cut]), string(body[cut+1:])
	expires, err := strconv.ParseInt(expiresRaw, 10, 64)
	if err != nil || now.Unix() > expires {
		return "", ErrBadLink
	}

	return payload, nil
}

// DeriveKey tire d'un secret une cle propre a un usage : deux usages du meme
// secret ne partagent pas leur cle, un jeton de l'un ne vaut rien pour
// l'autre.
func DeriveKey(secret []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}
