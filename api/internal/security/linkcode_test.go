package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLinkCodeTireHuitCaracteresCrockford(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code, err := NewLinkCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 8 || strings.ContainsAny(code, "ILOU-") {
			t.Fatalf("code %q : 8 caracteres Crockford attendus", code)
		}
		if normalized, ok := NormalizeLinkCode(code); !ok || normalized != code {
			t.Fatalf("un code tire doit se relire tel quel : %q -> %q, %v", code, normalized, ok)
		}
		seen[code] = true
	}
	if len(seen) < 199 {
		t.Errorf("%d codes distincts sur 200 : le tirage n'est pas aleatoire", len(seen))
	}
}

func TestNormalizeLinkCodeTolereCasseTiretEtConfusions(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{"K7QM-4XZD", "K7QM4XZD", true},
		{"k7qm4xzd", "K7QM4XZD", true},
		{" k7qm - 4xzd ", "K7QM4XZD", true},
		{"O0IL-1ABC", "00111ABC", true},
		{"K7QM-4XZ", "", false},
		{"K7QM-4XZDE", "", false},
		{"K7QM-4XZU", "", false},
		{"K7QM_4XZD", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeLinkCode(c.input)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeLinkCode(%q) = %q, %v ; attendu %q, %v", c.input, got, ok, c.want, c.ok)
		}
	}
}

func TestFormatLinkCodeSepareDeuxGroupes(t *testing.T) {
	if got := FormatLinkCode("K7QM4XZD"); got != "K7QM-4XZD" {
		t.Errorf("FormatLinkCode = %q", got)
	}
}

func TestHashLinkCodeDependDeLaCle(t *testing.T) {
	a := HashLinkCode([]byte("cle-a"), "K7QM4XZD")
	b := HashLinkCode([]byte("cle-b"), "K7QM4XZD")
	if bytes.Equal(a, b) {
		t.Error("deux cles differentes donnent la meme empreinte : la base seule suffirait a retrouver un code")
	}
	if !bytes.Equal(a, HashLinkCode([]byte("cle-a"), "K7QM4XZD")) {
		t.Error("l'empreinte doit etre stable pour une meme cle")
	}
}
