package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// ErrSealed : un secret chiffre illisible, ou chiffre avec une autre cle —
// JWT_SECRET a change depuis.
var ErrSealed = errors.New("secret illisible : la cle a change ?")

// Seal chiffre un secret a garder en base (AES-256-GCM). La cle vient de
// DeriveKey : chaque usage a la sienne.
func Seal(key []byte, plain string) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

// Open dechiffre ce que Seal a produit.
func Open(key, sealed []byte) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", ErrSealed
	}
	plain, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrSealed
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("cle de 32 octets attendue")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
