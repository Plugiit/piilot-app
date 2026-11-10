package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Jetons et controles du serveur d'autorisation OAuth 2.1 qui connecte Claude
// a un compte : codes d'autorisation, jetons d'acces et de renouvellement,
// preuve PKCE et comparaison des adresses de retour.

// Prefixes des jetons. Reconnaissables, ils permettent aux outils de detection
// de secrets de les reperer, et a la garde de refuser sans lire la base un
// jeton qui n'a pas la bonne forme.
const (
	OAuthAccessPrefix  = "piilot_at_"
	OAuthRefreshPrefix = "piilot_rt_"
	OAuthCodePrefix    = "piilot_ac_"
)

// oauthTokenBytes : 256 bits d'alea, comme les cles du bot. Le hachage rapide
// suffit donc : rien a deviner.
const oauthTokenBytes = 32

// NewOAuthToken tire un jeton du prefixe donne et rend sa forme claire
// (envoyee une fois, jamais stockee) et son empreinte (stockee).
func NewOAuthToken(prefix string) (raw string, hash []byte, err error) {
	buf := make([]byte, oauthTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generation du jeton : %w", err)
	}
	raw = prefix + base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// OAuthTokenHash rend l'empreinte d'un jeton presente, ou faux s'il n'a pas la
// forme attendue pour ce prefixe.
func OAuthTokenHash(prefix, raw string) ([]byte, bool) {
	if len(raw) != len(prefix)+base64.RawURLEncoding.EncodedLen(oauthTokenBytes) || !strings.HasPrefix(raw, prefix) {
		return nil, false
	}
	if _, err := base64.RawURLEncoding.DecodeString(raw[len(prefix):]); err != nil {
		return nil, false
	}
	return HashRefreshToken(raw), true
}

// ValidPKCEChallenge dit si un defi S256 a la bonne forme : l'empreinte
// SHA-256 d'un verificateur, en base64url sans remplissage (43 caracteres).
func ValidPKCEChallenge(challenge string) bool {
	if len(challenge) != base64.RawURLEncoding.EncodedLen(sha256.Size) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(challenge)
	return err == nil
}

// VerifyPKCE controle le verificateur presente au jeton contre le defi recu a
// l'autorisation (RFC 7636, methode S256 seule). Le verificateur fait 43 a 128
// caracteres non reserves.
func VerifyPKCE(challenge, verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, r := range verifier {
		if !isUnreserved(r) {
			return false
		}
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func isUnreserved(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
		r == '-' || r == '.' || r == '_' || r == '~'
}

// MatchRedirectURI dit si l'adresse de retour demandee correspond a une
// adresse enregistree.
//
// Egalite stricte, sauf pour une application native qui ecoute en local : son
// port change a chaque session, il est donc ignore (RFC 8252, 7.3). La regle
// vaut pour 127.0.0.1 et [::1], et aussi pour localhost, que Claude Code
// declare. Tout le reste doit correspondre : schema http, meme hote, meme
// chemin, ni identifiants, ni requete, ni fragment.
func MatchRedirectURI(registered, requested string) bool {
	if registered == requested {
		return true
	}
	reg, err := url.Parse(registered)
	if err != nil {
		return false
	}
	req, err := url.Parse(requested)
	if err != nil {
		return false
	}
	if reg.Scheme != "http" || req.Scheme != "http" || !isLoopbackHost(reg.Hostname()) {
		return false
	}
	if req.Opaque != "" || req.User != nil || req.RawQuery != "" || req.Fragment != "" || req.RawFragment != "" {
		return false
	}
	if req.Hostname() != reg.Hostname() || req.EscapedPath() != reg.EscapedPath() {
		return false
	}
	if port := req.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}

// IsLoopbackRedirect dit si une adresse de retour vise la machine locale :
// l'ecran de consentement le signale, n'importe quel programme local pouvant
// ecouter ce port.
func IsLoopbackRedirect(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "http" && isLoopbackHost(u.Hostname())
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
