package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeUmami joue une instance Umami 2.x ou 3.x, et sait faire expirer ses
// jetons.
type fakeUmami struct {
	mu      sync.Mutex
	version int
	logins  int
	valid   string
	apiKey  string
	prefix  string // "/api" pour une instance, "" pour Cloud
	// Des equipes : includeTeams est ignore, comme le font certaines
	// versions pour un simple membre.
	teams bool
}

func (f *fakeUmami) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, f.prefix)

	if path == "/auth/login" && r.Method == http.MethodPost {
		var body struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Username != "admin" || body.Password != "umami" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.logins++
		f.valid = fmt.Sprintf("jeton-%d", f.logins)
		_ = json.NewEncoder(w).Encode(map[string]any{"token": f.valid, "user": map[string]string{"username": "admin"}})
		return
	}

	authorized := (f.apiKey != "" && r.Header.Get("x-umami-api-key") == f.apiKey) ||
		(f.valid != "" && r.Header.Get("Authorization") == "Bearer "+f.valid)
	if !authorized {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	switch {
	case path == "/teams" && f.teams:
		_, _ = w.Write([]byte(`{"data":[{"id":"t1","name":"La Broche"},{"id":"t2","name":"Ridia"}],"count":2,"page":1,"pageSize":100}`))
	case path == "/teams/t1/websites" && f.teams:
		_, _ = w.Write([]byte(`{"data":[{"id":"w2","name":"Boutique","domain":"shop.example"},{"id":"w3","name":"La Broche","domain":"labroche.example"}],"count":2,"page":1,"pageSize":100}`))
	case path == "/teams/t2/websites" && f.teams:
		w.WriteHeader(http.StatusForbidden)
	case path == "/websites":
		_, _ = w.Write([]byte(`{"data":[{"id":"w1","name":"Vitrine","domain":"atelier.example"},{"id":"w2","name":"Boutique","domain":"shop.example"}],"count":2,"page":1,"pageSize":100}`))
	case strings.HasSuffix(path, "/stats"):
		if q.Get("startAt") == "" || q.Get("endAt") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if f.version == 2 {
			_, _ = w.Write([]byte(`{"pageviews":{"value":120,"prev":100},"visitors":{"value":40,"prev":50},"visits":{"value":55,"prev":60},"bounces":{"value":20,"prev":30},"totaltime":{"value":3600,"prev":3000}}`))
		} else {
			_, _ = w.Write([]byte(`{"pageviews":120,"visitors":40,"visits":55,"bounces":20,"totaltime":3600,"comparison":{"pageviews":100,"visitors":50,"visits":60,"bounces":30,"totaltime":3000}}`))
		}
	case strings.HasSuffix(path, "/metrics"):
		typ := q.Get("type")
		if (typ == "url" && f.version != 2) || (typ == "path" && f.version == 2) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`[{"x":"/","y":80},{"x":"/contact","y":"12"}]`))
	case strings.HasSuffix(path, "/pageviews"):
		if q.Get("unit") != "day" || q.Get("timezone") != "Europe/Paris" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"pageviews":[{"x":"2026-10-11 00:00:00","y":30},{"x":"2026-10-12 00:00:00","y":90}],"sessions":[{"x":"2026-10-12 00:00:00","y":25},{"x":"2026-10-11 00:00:00","y":15}]}`))
	case strings.HasSuffix(path, "/active"):
		if f.version == 2 {
			_, _ = w.Write([]byte(`{"x":3}`))
		} else {
			_, _ = w.Write([]byte(`{"visitors":3}`))
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeUmami) set(fn func(*fakeUmami)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

var week = Range{Start: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}

func TestUneInstance2xSeLitEtSeReconnait(t *testing.T) {
	fake := &fakeUmami{version: 2, prefix: "/api"}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	var changes [][2]int
	u := &Umami{BaseURL: srv.URL + "/", Username: "admin", Password: "umami", OnVersion: func(p, d int) { changes = append(changes, [2]int{p, d}) }}
	ctx := context.Background()

	sites, err := u.ListWebsites(ctx)
	if err != nil || len(sites) != 2 || sites[1].Domain != "shop.example" {
		t.Fatalf("sites : %+v, %v", sites, err)
	}
	stats, err := u.Stats(ctx, "w1", week)
	if err != nil {
		t.Fatal(err)
	}
	want := Comparison{Current: Totals{40, 55, 120, 20, 3600}, Previous: Totals{50, 60, 100, 30, 3000}}
	if stats != want {
		t.Errorf("chiffres : %+v", stats)
	}
	if u.CurrentVersion() != 2 || len(changes) != 1 || changes[0] != [2]int{0, 2} {
		t.Errorf("version : %d, changements %v", u.CurrentVersion(), changes)
	}
	// En 2.x, le classement des pages s'appelle « url ».
	pages, err := u.Metrics(ctx, "w1", week, Page, 10)
	if err != nil || len(pages) != 2 || pages[1].Count != 12 {
		t.Fatalf("pages : %+v, %v", pages, err)
	}
	if n, err := u.Active(ctx, "w1"); err != nil || n != 3 {
		t.Errorf("visiteurs actifs : %d, %v", n, err)
	}
}

func TestUneMiseAJourVers3xRelanceLaDetection(t *testing.T) {
	fake := &fakeUmami{version: 2, prefix: "/api"}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	var changes [][2]int
	u := &Umami{BaseURL: srv.URL, Username: "admin", Password: "umami", Version: 2, OnVersion: func(p, d int) { changes = append(changes, [2]int{p, d}) }}
	ctx := context.Background()

	// L'instance passe en 3.x : le classement des pages sous son nom 2.x est
	// refuse, l'autre nom est essaye, et la version corrigee.
	fake.set(func(f *fakeUmami) { f.version = 3 })
	pages, err := u.Metrics(ctx, "w1", week, Page, 10)
	if err != nil || len(pages) != 2 {
		t.Fatalf("pages apres la mise a jour : %+v, %v", pages, err)
	}
	if u.CurrentVersion() != 3 || len(changes) != 1 || changes[0] != [2]int{2, 3} {
		t.Fatalf("version : %d, changements %v", u.CurrentVersion(), changes)
	}

	// Et /stats au format 3.x se lit sans detour.
	stats, err := u.Stats(ctx, "w1", week)
	if err != nil || stats.Current.Pageviews != 120 || stats.Previous.Visitors != 50 {
		t.Errorf("chiffres 3.x : %+v, %v", stats, err)
	}
	if n, _ := u.Active(ctx, "w1"); n != 3 {
		t.Errorf("visiteurs actifs 3.x : %d", n)
	}
}

func TestUneVersionChoisieALaMainNeBougePas(t *testing.T) {
	fake := &fakeUmami{version: 3, prefix: "/api"}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	called := false
	u := &Umami{BaseURL: srv.URL, Username: "admin", Password: "umami", Version: 3, Pinned: true, OnVersion: func(int, int) { called = true }}
	// La forme 3.x se lit quelle que soit la version retenue ; la version
	// choisie a la main reste.
	if _, err := u.Stats(context.Background(), "w1", week); err != nil {
		t.Fatal(err)
	}
	fake.set(func(f *fakeUmami) { f.version = 2 })
	if _, err := u.Stats(context.Background(), "w1", week); err != nil {
		t.Fatal(err)
	}
	if called || u.CurrentVersion() != 3 {
		t.Errorf("version choisie modifiee : %d", u.CurrentVersion())
	}
}

func TestUnJetonExpireEstRenouvele(t *testing.T) {
	fake := &fakeUmami{version: 3, prefix: "/api"}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	var tokens []string
	u := &Umami{BaseURL: srv.URL, Username: "admin", Password: "umami", OnToken: func(tok string) { tokens = append(tokens, tok) }}
	ctx := context.Background()
	if _, err := u.ListWebsites(ctx); err != nil {
		t.Fatal(err)
	}
	// Umami a redemarre, le jeton ne vaut plus rien : une connexion de plus,
	// et la requete passe.
	fake.set(func(f *fakeUmami) { f.valid = "autre" })
	if _, err := u.ListWebsites(ctx); err != nil {
		t.Fatalf("apres expiration : %v", err)
	}
	if len(tokens) != 2 || tokens[1] != "jeton-2" {
		t.Errorf("jetons : %v", tokens)
	}

	// Un jeton garde d'un demarrage precedent sert tel quel.
	reused := &Umami{BaseURL: srv.URL, Username: "admin", Password: "umami"}
	reused.SetToken("jeton-2")
	if _, err := reused.ListWebsites(ctx); err != nil || fake.logins != 2 {
		t.Errorf("jeton repris : %v, %d connexions", err, fake.logins)
	}

	bad := &Umami{BaseURL: srv.URL, Username: "admin", Password: "faux"}
	if _, err := bad.ListWebsites(ctx); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("mauvais mot de passe : %v", err)
	}
}

func TestUmamiCloudParCleDAPI(t *testing.T) {
	fake := &fakeUmami{version: 3, prefix: "/v1", apiKey: "cle-cloud"}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	u := &Umami{BaseURL: srv.URL + "/v1", APIKey: "cle-cloud"}
	if sites, err := u.ListWebsites(context.Background()); err != nil || len(sites) != 2 {
		t.Fatalf("Cloud : %+v, %v", sites, err)
	}
	if _, err := (&Umami{BaseURL: srv.URL + "/v1", APIKey: "fausse"}).ListWebsites(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("mauvaise cle : %v", err)
	}
}

func TestLaCourbeAligneSesDeuxSeries(t *testing.T) {
	fake := &fakeUmami{version: 3, prefix: "/api"}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	u := &Umami{BaseURL: srv.URL, Username: "admin", Password: "umami"}
	points, err := u.Timeseries(context.Background(), "w1", week, Day, "Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	want := []Point{{At: "2026-10-11T00:00:00", Pageviews: 30, Visitors: 15}, {At: "2026-10-12T00:00:00", Pageviews: 90, Visitors: 25}}
	if len(points) != 2 || points[0] != want[0] || points[1] != want[1] {
		t.Errorf("courbe : %+v", points)
	}
}

func TestLesFormesDeReponsesConnues(t *testing.T) {
	if _, _, err := ParseStats(map[string]json.RawMessage{"visitors": json.RawMessage(`1`)}); err == nil {
		t.Error("/stats sans pages vues accepte")
	}
	sites, more, err := parseWebsites(json.RawMessage(`[{"id":"a","name":"A","domain":"a.example"}]`))
	if err != nil || len(sites) != 1 || more {
		t.Errorf("tableau nu : %+v, %v, %v", sites, more, err)
	}
	_, more, _ = parseWebsites(json.RawMessage(`{"data":[{"id":"a"}],"count":250,"page":1,"pageSize":100}`))
	if !more {
		t.Error("une page incomplete n'appelle pas la suivante")
	}
	for in, want := range map[string]string{
		"2026-10-12 00:00:00":      "2026-10-12T00:00:00",
		"2026-10-12T14:00:00.000Z": "2026-10-12T14:00:00",
		"2026-10-12":               "2026-10-12T00:00:00",
	} {
		if got := normalizeAt(in); got != want {
			t.Errorf("%q : %q", in, got)
		}
	}
}

func TestLesSitesDesEquipesSontLus(t *testing.T) {
	fake := &fakeUmami{version: 3, prefix: "/api", teams: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	u := &Umami{BaseURL: srv.URL, Username: "admin", Password: "umami"}
	sites, err := u.ListWebsites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Les siens, puis ceux des equipes, sans doublon ; une equipe illisible
	// est passee.
	var ids []string
	for _, s := range sites {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "w1,w2,w3" {
		t.Errorf("sites : %v", ids)
	}
}
