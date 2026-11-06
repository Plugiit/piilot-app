package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

// BotKeyPrefix ouvre chaque cle du bot. Reconnaissable, il permet aux outils de
// detection de secrets de reperer une cle collee dans un depot ou un ticket.
const BotKeyPrefix = "piilot_bot_"

// botKeyBytes : 256 bits d'alea, comme les jetons de rafraichissement. Le
// hachage rapide qui suit suffit donc : rien a deviner.
const botKeyBytes = 32

// botKeyLen : le prefixe et 32 octets en base64url sans remplissage.
var botKeyLen = len(BotKeyPrefix) + base64.RawURLEncoding.EncodedLen(botKeyBytes)

// NewBotKey tire une cle et rend sa forme claire (montree une seule fois,
// jamais stockee) et son empreinte (stockee).
func NewBotKey() (raw string, hash []byte, err error) {
	buf := make([]byte, botKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generation de la cle du bot : %w", err)
	}
	raw = BotKeyPrefix + base64.RawURLEncoding.EncodeToString(buf)
	return raw, HashRefreshToken(raw), nil
}

// BotKeyHash rend l'empreinte d'une cle presentee, ou faux si elle n'a pas la
// forme d'une cle du bot. Un jeton d'une autre forme n'atteint pas la base.
func BotKeyHash(raw string) ([]byte, bool) {
	if len(raw) != botKeyLen || !strings.HasPrefix(raw, BotKeyPrefix) {
		return nil, false
	}
	if _, err := base64.RawURLEncoding.DecodeString(raw[len(BotKeyPrefix):]); err != nil {
		return nil, false
	}
	return HashRefreshToken(raw), true
}
