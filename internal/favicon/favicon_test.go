package favicon

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsPrivate couvre le filtre qui empeche l'API de sortir vers son propre
// reseau.
//
// C'est le garde-fou contre la SSRF : une app dont l'adresse resout vers une
// machine interne ferait sonder ce reseau depuis l'API. Les cas negatifs
// comptent autant — bloquer une adresse publique casserait la fonction.
func TestIsPrivate(t *testing.T) {
	cas := []struct {
		ip      string
		attendu bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		// L'adresse des metadonnees chez la plupart des hebergeurs : celle qui
		// sert des identifiants a qui sait la demander.
		{"169.254.169.254", true},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"fd00::1", true},
		{"fe80::1", true},

		{"1.1.1.1", false},
		{"8.8.8.8", false},
		{"142.250.75.238", false},
		{"2606:4700:4700::1111", false},
	}

	for _, c := range cas {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("adresse de test illisible : %s", c.ip)
		}

		if got := isPrivate(ip); got != c.attendu {
			t.Errorf("isPrivate(%s) = %v, attendu %v", c.ip, got, c.attendu)
		}
	}
}

// TestFetchRefuseReseauLocal verifie que le refus tient de bout en bout, et pas
// seulement dans la fonction de filtrage : c'est le dialer qui doit couper, y
// compris quand l'adresse est parfaitement valide par ailleurs.
func TestFetchRefuseReseauLocal(t *testing.T) {
	// Un vrai serveur, sur la boucle locale : exactement ce qu'une SSRF
	// cherche a joindre.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	defer server.Close()

	if _, err := New().Fetch(context.Background(), server.URL); err == nil {
		t.Fatal("une adresse de la boucle locale a ete jointe ; elle devait etre refusee")
	}
}

// TestParseSize lit l'attribut qui dit la taille d'une icone declaree.
func TestParseSize(t *testing.T) {
	cas := map[string]int{
		"32x32":         32,
		"16x16 32x32":   32,
		"180X180":       180,
		"any":           0,
		"":              0,
		"grand":         0,
		"48x48 128x128": 128,
	}

	for entree, attendu := range cas {
		if got := parseSize(entree); got != attendu {
			t.Errorf("parseSize(%q) = %d, attendu %d", entree, got, attendu)
		}
	}
}
