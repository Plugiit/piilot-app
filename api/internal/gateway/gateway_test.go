package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeApp est une instance de l'application : elle dit qui elle est et ce
// qu'elle a recu.
type fakeApp struct {
	name     string
	started  time.Time
	draining atomic.Bool
	srv      *httptest.Server
}

func newFakeApp(t *testing.T, name string, started time.Time) *fakeApp {
	t.Helper()
	a := &fakeApp{name: name, started: started}
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/live" {
			if a.draining.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"started_at": a.started})
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Instance", a.name)
		w.Header().Set("X-Seen-Host", r.Host)
		w.Header().Set("X-Seen-For", r.Header.Get("X-Forwarded-For"))
		_, _ = w.Write(body)
	}))
	t.Cleanup(a.srv.Close)
	return a
}

func (a *fakeApp) addr() string { return strings.TrimPrefix(a.srv.URL, "http://") }

// testGateway pointe la passerelle vers une liste d'instances modifiable.
type testGateway struct {
	*Gateway
	mu    sync.Mutex
	addrs []string
}

func newTestGateway(t *testing.T, hold time.Duration) *testGateway {
	t.Helper()
	tg := &testGateway{}
	tg.Gateway = New(Config{Upstream: "server:8080", ProbeInterval: 50 * time.Millisecond, Hold: hold},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	tg.resolve = func(context.Context, string) ([]string, error) {
		tg.mu.Lock()
		defer tg.mu.Unlock()
		return append([]string{}, tg.addrs...), nil
	}
	return tg
}

func (tg *testGateway) set(addrs ...string) {
	tg.mu.Lock()
	tg.addrs = addrs
	tg.mu.Unlock()
}

// settle passe assez de sondes pour qu'une instance neuve soit elue.
func (tg *testGateway) settle() {
	for range 3 {
		tg.refresh(context.Background())
	}
}

func (tg *testGateway) do(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "team.agence.fr"
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	tg.ServeHTTP(rec, req)
	return rec.Result()
}

func TestLaBasculeVaALaPlusRecenteEtEcarteCelleQuiSArrete(t *testing.T) {
	now := time.Now()
	old := newFakeApp(t, "ancienne", now.Add(-time.Hour))
	tg := newTestGateway(t, time.Second)

	tg.set(old.addr())
	tg.settle()
	if got := tg.do(t, http.MethodGet, "/", "").Header.Get("X-Instance"); got != "ancienne" {
		t.Fatalf("avant la mise a jour : %q", got)
	}

	// La nouvelle version demarre a cote : une premiere sonde ne suffit pas.
	fresh := newFakeApp(t, "nouvelle", now)
	tg.set(old.addr(), fresh.addr())
	tg.refresh(context.Background())
	if got := tg.do(t, http.MethodGet, "/", "").Header.Get("X-Instance"); got != "ancienne" {
		t.Fatalf("bascule sur une seule sonde : %q", got)
	}

	tg.settle()
	if got := tg.do(t, http.MethodGet, "/", "").Header.Get("X-Instance"); got != "nouvelle" {
		t.Fatalf("apres la bascule : %q", got)
	}

	// Si la nouvelle s'arrete, l'ancienne, toujours saine, reprend.
	fresh.draining.Store(true)
	tg.refresh(context.Background())
	if got := tg.do(t, http.MethodGet, "/", "").Header.Get("X-Instance"); got != "ancienne" {
		t.Fatalf("retour a l'ancienne : %q", got)
	}
}

func TestUneRequeteAttendQuUneInstanceSoitPrete(t *testing.T) {
	tg := newTestGateway(t, 3*time.Second)
	app := newFakeApp(t, "unique", time.Now())

	go func() {
		time.Sleep(200 * time.Millisecond)
		tg.set(app.addr())
		tg.settle()
	}()

	res := tg.do(t, http.MethodGet, "/pm", "")
	if res.StatusCode != http.StatusOK || res.Header.Get("X-Instance") != "unique" {
		t.Fatalf("statut %d, instance %q", res.StatusCode, res.Header.Get("X-Instance"))
	}
}

func TestSansInstanceLaPageDeMaintenancePuisUn503(t *testing.T) {
	tg := newTestGateway(t, 100*time.Millisecond)

	page := tg.do(t, http.MethodGet, "/pm/projets", "")
	body, _ := io.ReadAll(page.Body)
	if page.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), "Mise à jour en cours") {
		t.Fatalf("page : %d %s", page.StatusCode, body)
	}

	api := tg.do(t, http.MethodPost, "/api/v1/admin/crm/clients", "{}")
	var payload struct{ Code string }
	_ = json.NewDecoder(api.Body).Decode(&payload)
	if api.StatusCode != http.StatusServiceUnavailable || payload.Code != "MAINTENANCE" || api.Header.Get("Retry-After") == "" {
		t.Fatalf("api : %d %+v", api.StatusCode, payload)
	}

	status := tg.do(t, http.MethodGet, statusPath, "")
	var s struct{ Ready bool }
	_ = json.NewDecoder(status.Body).Decode(&s)
	if status.StatusCode != http.StatusOK || s.Ready {
		t.Fatalf("sonde de la passerelle : %d %+v", status.StatusCode, s)
	}
}

func TestUneRequeteQuiNaPasPuPartirEstRenvoyeeAvecSonCorps(t *testing.T) {
	alive := newFakeApp(t, "vivante", time.Now().Add(-time.Hour))

	// Une instance elue puis disparue sans prevenir : son port ne repond plus.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gone := ln.Addr().String()
	_ = ln.Close()

	tg := newTestGateway(t, time.Second)
	tg.set(alive.addr())
	tg.settle()
	tg.mu.Lock()
	tg.addrs = []string{alive.addr(), gone}
	tg.mu.Unlock()
	tg.Gateway.mu.Lock()
	tg.instances[gone] = &instance{addr: gone, healthy: true, successes: 5, started: time.Now()}
	tg.elect()
	tg.Gateway.mu.Unlock()

	res := tg.do(t, http.MethodPost, "/api/v1/x", `{"nom":"Danone"}`)
	body, _ := io.ReadAll(res.Body)
	if res.Header.Get("X-Instance") != "vivante" || string(body) != `{"nom":"Danone"}` {
		t.Fatalf("instance %q, corps %q", res.Header.Get("X-Instance"), body)
	}
}

func TestLHoteEtLAppelantSontTransmis(t *testing.T) {
	app := newFakeApp(t, "unique", time.Now())
	tg := newTestGateway(t, time.Second)
	tg.set(app.addr())
	tg.settle()

	res := tg.do(t, http.MethodGet, "/", "")
	if res.Header.Get("X-Seen-Host") != "team.agence.fr" {
		t.Errorf("hote : %q", res.Header.Get("X-Seen-Host"))
	}
	// httptest pose 192.0.2.1 comme adresse distante : une adresse publique.
	if res.Header.Get("X-Seen-For") != "192.0.2.1" {
		t.Errorf("appelant : %q", res.Header.Get("X-Seen-For"))
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name, remote string
		forwarded    []string
		want         string
	}{
		{"appel direct", "203.0.113.7:5000", nil, "203.0.113.7"},
		{"appel direct qui s'invente une adresse", "203.0.113.7:5000", []string{"1.1.1.1"}, "203.0.113.7"},
		{"derriere Traefik", "172.18.0.3:5000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"derriere nginx qui ajoute", "127.0.0.1:5000", []string{"6.6.6.6, 203.0.113.7"}, "203.0.113.7"},
		{"deux proxys", "10.0.0.2:5000", []string{"203.0.113.7, 10.0.0.9"}, "203.0.113.7"},
		{"en-tete illisible", "10.0.0.2:5000", []string{"pas-une-ip"}, "10.0.0.2"},
	}
	for _, c := range cases {
		if got := ClientIP(c.remote, c.forwarded); got != c.want {
			t.Errorf("%s : %q, attendu %q", c.name, got, c.want)
		}
	}
}
