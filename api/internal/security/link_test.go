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
