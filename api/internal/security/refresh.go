package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
)

// refreshTokenBytes : 32 octets d'alea, soit 256 bits. Deviner un tel jeton est
// hors de portee, ce qui autorise le hachage rapide ci-dessous.
const refreshTokenBytes = 32

// NewRefreshToken tire un jeton de rafraichissement et retourne sa forme claire
// (envoyee au client, jamais stockee) et son empreinte (stockee, jamais
// envoyee).
func NewRefreshToken() (raw string, hash []byte, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generation du jeton de rafraichissement : %w", err)
	}

	raw = base64.RawURLEncoding.EncodeToString(buf)

	return raw, HashRefreshToken(raw), nil
}

// HashRefreshToken calcule l'empreinte stockee en base.
//
// SHA-256 et non bcrypt, contrairement aux mots de passe : bcrypt protege une
// entree devinable, ce qu'un tirage de 256 bits n'est pas. Un hachage lent ici
// ne couvrirait aucun risque et ralentirait chaque rafraichissement.
func HashRefreshToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// EqualRefreshHash compare deux empreintes en temps constant.
//
// La recherche en base se fait deja par index unique sur l'empreinte ; cette
// fonction sert aux comparaisons faites en memoire, ou une comparaison naive
// fuirait le prefixe correct octet par octet.
func EqualRefreshHash(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
