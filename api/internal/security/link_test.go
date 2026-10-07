package security

import (
	"testing"
	"time"
)

func TestSignLinkEtVerifyLink(t *testing.T) {
	secret := []byte("une-cle")
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	token := SignLink(secret, "version:abc|user:def", now.Add(time.Hour))

	payload, err := VerifyLink(secret, token, now)
	if err != nil || payload != "version:abc|user:def" {
		t.Fatalf("relecture : %q, %v", payload, err)
	}
	if _, err := VerifyLink(secret, token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("jeton perime accepte")
	}
	if _, err := VerifyLink([]byte("autre"), token, now); err == nil {
		t.Fatal("mauvaise cle acceptee")
	}
	if _, err := VerifyLink(secret, token[:len(token)-2]+"zz", now); err == nil {
		t.Fatal("jeton altere accepte")
	}
	if _, err := VerifyLink(secret, "n'importe quoi", now); err == nil {
		t.Fatal("jeton illisible accepte")
	}
}

func TestSealOpen(t *testing.T) {
	key := DeriveKey([]byte("secret"), "imap")
	sealed, err := Seal(key, "mot de passé")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Open(key, sealed); err != nil || got != "mot de passé" {
		t.Fatalf("relecture : %q %v", got, err)
	}
	if _, err := Open(DeriveKey([]byte("autre"), "imap"), sealed); err == nil {
		t.Fatal("autre cle acceptee")
	}
}
