package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewerCompareLesVersions(t *testing.T) {
	cas := []struct {
		candidate, current string
		want               bool
	}{
		{"0.5.0", "0.4.1", true},
		{"0.4.10", "0.4.9", true}, // numerique, pas alphabetique
		{"1.0.0", "0.9.9", true},
		{"0.4.1", "0.4.1", false},
		{"0.4.0", "0.4.1", false},
		{"1.0.0", "1.0.0-rc.2", true},
		{"1.0.0-rc.2", "1.0.0", false},
		{"v0.5.0", "0.4.1", true},
		{"0.5.0", "dev", false}, // instance hors release : rien a proposer
		{"n'importe quoi", "0.4.1", false},
	}

	for _, c := range cas {
		if got := Newer(c.candidate, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, attendu %v", c.candidate, c.current, got, c.want)
		}
	}
}

func TestLatestReleaseLitLeTagSansLePrefixe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/Plugiit/piilot-app/releases/latest" {
			t.Errorf("chemin = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"tag_name":"v0.5.0","name":"Piilot v0.5.0 — Comptes","html_url":"https://example.test/r","published_at":"2026-10-10T10:00:00Z"}`))
	}))
	defer srv.Close()

	c := New()
	c.githubAPI = srv.URL

	got, err := c.LatestRelease(context.Background(), "Plugiit/piilot-app")
	if err != nil {
		t.Fatalf("LatestRelease : %v", err)
	}
	if got.Version != "0.5.0" || got.URL != "https://example.test/r" || got.PublishedAt == nil {
		t.Errorf("release = %+v", got)
	}
}
