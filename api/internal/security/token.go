// Package security regroupe la signature des jetons et les primitives
// d'authentification.
package security

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims transporte l'identite de l'appelant. Volontairement minimal : tout ce
// qui peut changer (permissions fines, nom) est relu en base, pour qu'une
// revocation prenne effet immediatement au lieu d'attendre l'expiration.
type Claims struct {
	UserID uuid.UUID `json:"sub"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// TokenSigner signe et verifie les jetons d'acces (HMAC-SHA256).
type TokenSigner struct {
	secret []byte
	issuer string
}

// NewTokenSigner construit le signeur a partir du secret applicatif.
func NewTokenSigner(secret []byte, issuer string) *TokenSigner {
	return &TokenSigner{secret: secret, issuer: issuer}
}

// Sign emet un jeton d'acces pour l'utilisateur donne.
func (s *TokenSigner) Sign(userID uuid.UUID, role string, ttl time.Duration) (string, error) {
	now := time.Now()

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.NewString(),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("signature du jeton : %w", err)
	}

	return token, nil
}

// Verify valide la signature et l'expiration, puis retourne les claims.
//
// L'algorithme attendu est impose explicitement : sans cette contrainte, un
// jeton forge avec alg=none ou alg=HS256-sur-cle-publique passerait la
// verification (confusion d'algorithme).
func (s *TokenSigner) Verify(raw string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return s.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(s.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("jeton invalide : %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("jeton invalide")
	}

	return claims, nil
}
