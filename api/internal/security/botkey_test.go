package security

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewBotKeyPorteLePrefixeEtSeRelit(t *testing.T) {
	raw, hash, err := NewBotKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, BotKeyPrefix) {
		t.Fatalf("cle sans le prefixe %q", BotKeyPrefix)
	}
	got, ok := BotKeyHash(raw)
	if !ok || !bytes.Equal(got, hash) {
		t.Fatal("une cle tiree doit se relire avec la meme empreinte")
	}
	other, _, _ := NewBotKey()
	if other == raw {
		t.Fatal("deux tirages identiques")
	}
}

func TestBotKeyHashRefuseCeQuiNEstPasUneCle(t *testing.T) {
	raw, _, err := NewBotKey()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"vide":             "",
		"autre prefixe":    "pgt_bot_" + raw[len(BotKeyPrefix):],
		"tronquee":         raw[:len(raw)-1],
		"trop longue":      raw + "A",
		"hors base64url":   raw[:len(raw)-1] + "+",
		"jeton de session": "eyJhbGciOiJIUzI1NiJ9.e30.signature",
	}
	for name, input := range cases {
		if _, ok := BotKeyHash(input); ok {
			t.Errorf("%s : accepte", name)
		}
	}
}
