package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func signer() *TokenSigner {
	return NewTokenSigner([]byte("secret-de-test-suffisamment-long-32"), "plugiit-api")
}

func TestSignPuisVerifyRestitueLesClaims(t *testing.T) {
	s := signer()
	userID := uuid.New()

	raw, err := s.Sign(userID, "admin", time.Minute)
	if err != nil {
		t.Fatalf("Sign : %v", err)
	}

	claims, err := s.Verify(raw)
	if err != nil {
		t.Fatalf("Verify : %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %v, attendu %v", claims.UserID, userID)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q, attendu %q", claims.Role, "admin")
	}
}

func TestVerifyRejetteUnJetonExpire(t *testing.T) {
	s := signer()

	raw, err := s.Sign(uuid.New(), "admin", -time.Minute)
	if err != nil {
		t.Fatalf("Sign : %v", err)
	}

	if _, err := s.Verify(raw); err == nil {
		t.Fatal("un jeton expire doit etre rejete")
	}
}

func TestVerifyRejetteUneAutreCle(t *testing.T) {
	raw, err := signer().Sign(uuid.New(), "admin", time.Minute)
	if err != nil {
		t.Fatalf("Sign : %v", err)
	}

	autre := NewTokenSigner([]byte("une-autre-cle-de-test-suffisamment-32"), "plugiit-api")
	if _, err := autre.Verify(raw); err == nil {
		t.Fatal("un jeton signe avec une autre cle doit etre rejete")
	}
}

// Confusion d'algorithme : un jeton non signe (alg=none) ne doit jamais passer.
func TestVerifyRejetteAlgNone(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{Role: "admin"})
	raw, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("forge du jeton : %v", err)
	}

	if _, err := signer().Verify(raw); err == nil {
		t.Fatal("alg=none doit etre rejete")
	}
}
