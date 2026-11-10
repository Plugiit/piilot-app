package security

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestJetonOAuthSeRelitAvecSonPrefixeSeulement(t *testing.T) {
	raw, hash, err := NewOAuthToken(OAuthAccessPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, OAuthAccessPrefix) {
		t.Fatalf("jeton sans le prefixe %q", OAuthAccessPrefix)
	}
	got, ok := OAuthTokenHash(OAuthAccessPrefix, raw)
	if !ok || !bytes.Equal(got, hash) {
		t.Fatal("un jeton tire doit se relire avec la meme empreinte")
	}
	if _, ok := OAuthTokenHash(OAuthRefreshPrefix, raw); ok {
		t.Fatal("un jeton d'acces ne vaut pas jeton de renouvellement")
	}
	for _, bad := range []string{"", OAuthAccessPrefix, raw + "x", raw[:len(raw)-1], OAuthAccessPrefix + strings.Repeat("!", 43)} {
		if _, ok := OAuthTokenHash(OAuthAccessPrefix, bad); ok {
			t.Fatalf("%q ne doit pas passer pour un jeton", bad)
		}
	}
}

func TestPKCEVerifieLeVerificateur(t *testing.T) {
	verifier := strings.Repeat("a1-._~", 8) // 48 caracteres non reserves
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !ValidPKCEChallenge(challenge) {
		t.Fatal("un defi S256 valide est refuse")
	}
	if !VerifyPKCE(challenge, verifier) {
		t.Fatal("le bon verificateur est refuse")
	}
	if VerifyPKCE(challenge, verifier+"b") {
		t.Fatal("un autre verificateur passe")
	}
	if VerifyPKCE(challenge, "court") {
		t.Fatal("un verificateur de moins de 43 caracteres passe")
	}
	if VerifyPKCE(challenge, strings.Repeat("é", 43)) {
		t.Fatal("un verificateur hors alphabet passe")
	}
	for _, bad := range []string{"", "plain", challenge + "A", strings.Repeat("+", 43)} {
		if ValidPKCEChallenge(bad) {
			t.Fatalf("defi %q accepte", bad)
		}
	}
}

func TestAdresseDeRetour(t *testing.T) {
	const claudeAI = "https://claude.ai/api/mcp/auth_callback"
	const local = "http://localhost/callback"
	const ip = "http://127.0.0.1/callback"

	cases := []struct {
		registered, requested string
		want                  bool
	}{
		{claudeAI, claudeAI, true},
		{claudeAI, "https://claude.ai/api/mcp/auth_callback?x=1", false},
		{claudeAI, "https://claude.ai:8443/api/mcp/auth_callback", false},
		{claudeAI, "https://evil.example/api/mcp/auth_callback", false},
		{local, "http://localhost:3118/callback", true},
		{local, "http://localhost/callback", true},
		{ip, "http://127.0.0.1:54321/callback", true},
		{local, "http://127.0.0.1:3118/callback", false},
		{local, "http://localhost.evil.com:3118/callback", false},
		{local, "http://localhost@evil.com/callback", false},
		{local, "http://user@localhost:3118/callback", false},
		{local, "https://localhost:3118/callback", false},
		{local, "http://localhost:3118/autre", false},
		{local, "http://localhost:3118/callback?code=1", false},
		{local, "http://localhost:3118/callback#x", false},
		{local, "http://localhost:99999/callback", false},
		{local, "http://localhost:0/callback", false},
	}
	for _, c := range cases {
		if got := MatchRedirectURI(c.registered, c.requested); got != c.want {
			t.Errorf("MatchRedirectURI(%q, %q) = %v, attendu %v", c.registered, c.requested, got, c.want)
		}
	}
}

func TestRetourLocalSignale(t *testing.T) {
	if !IsLoopbackRedirect("http://localhost:3118/callback") || !IsLoopbackRedirect("http://127.0.0.1:1/callback") {
		t.Fatal("un retour local doit etre signale")
	}
	if IsLoopbackRedirect("https://claude.ai/api/mcp/auth_callback") {
		t.Fatal("claude.ai n'est pas local")
	}
}
