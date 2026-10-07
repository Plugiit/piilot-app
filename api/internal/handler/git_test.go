package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestGithubSignatureValid(t *testing.T) {
	secret := "s3cret"
	body := []byte(`{"action":"opened"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	good := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !githubSignatureValid(secret, good, body) {
		t.Fatal("signature juste refusee")
	}
	if githubSignatureValid(secret, good, []byte(`{"action":"closed"}`)) {
		t.Fatal("corps modifie accepte")
	}
	if githubSignatureValid("autre", good, body) {
		t.Fatal("mauvais secret accepte")
	}
	if githubSignatureValid(secret, hex.EncodeToString(mac.Sum(nil)), body) {
		t.Fatal("signature sans prefixe acceptee")
	}
	if githubSignatureValid(secret, "", body) {
		t.Fatal("signature absente acceptee")
	}
}
