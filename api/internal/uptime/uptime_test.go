package uptime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Un extrait de /metrics de Kuma 2.x : identifiant dans les labels, une
// etiquette ajoutee, des champs « null ».
const kuma2 = `# HELP monitor_cert_days_remaining The number of days remaining until the certificate expires
# TYPE monitor_cert_days_remaining gauge
monitor_cert_days_remaining{env="prod",monitor_hostname="null",monitor_id="3",monitor_name="Vitrine \"Atelier\"",monitor_port="null",monitor_type="http",monitor_url="https://atelier.example"} 54
monitor_cert_is_valid{env="prod",monitor_hostname="null",monitor_id="3",monitor_name="Vitrine \"Atelier\"",monitor_port="null",monitor_type="http",monitor_url="https://atelier.example"} 1
monitor_response_time{env="prod",monitor_hostname="null",monitor_id="3",monitor_name="Vitrine \"Atelier\"",monitor_port="null",monitor_type="http",monitor_url="https://atelier.example"} 182
monitor_status{env="prod",monitor_hostname="null",monitor_id="3",monitor_name="Vitrine \"Atelier\"",monitor_port="null",monitor_type="http",monitor_url="https://atelier.example"} 1
monitor_status{monitor_hostname="db.example",monitor_id="7",monitor_name="Base",monitor_port="5432",monitor_type="port",monitor_url="null"} 0
monitor_response_time{monitor_hostname="db.example",monitor_id="7",monitor_name="Base",monitor_port="5432",monitor_type="port",monitor_url="null"} -1
monitor_uptime_ratio{monitor_id="3",monitor_name="Vitrine \"Atelier\"",window="1d"} 0.998
process_cpu_seconds_total 12.5
`

// Kuma 1.x : pas d'identifiant, le nom fait la cle.
const kuma1 = `monitor_status{monitor_name="Preprod",monitor_type="http",monitor_url="https://preprod.example",monitor_hostname="null",monitor_port="null"} 2
`

func TestLesMesuresDeKumaDonnentLesSites(t *testing.T) {
	samples, err := ParsePrometheus(strings.NewReader(kuma2))
	if err != nil {
		t.Fatal(err)
	}
	got := MonitorsFromSamples(samples)
	if len(got) != 2 {
		t.Fatalf("%d sites, attendu 2 : %+v", len(got), got)
	}

	base, site := got[0], got[1]
	if site.Key != "id:3" || site.Name != `Vitrine "Atelier"` || site.URL != "https://atelier.example" || site.Hostname != "" {
		t.Errorf("site : %+v", site)
	}
	if site.Status != Up || site.ResponseMS == nil || *site.ResponseMS != 182 || site.CertDays == nil || *site.CertDays != 54 || site.CertValid == nil || !*site.CertValid {
		t.Errorf("mesures du site : %+v", site)
	}
	if base.Key != "id:7" || base.Status != Down || base.ResponseMS != nil || base.URL != "" || base.Hostname != "db.example" {
		t.Errorf("base : %+v", base)
	}

	samples, err = ParsePrometheus(strings.NewReader(kuma1))
	if err != nil {
		t.Fatal(err)
	}
	if got := MonitorsFromSamples(samples); len(got) != 1 || got[0].Key != "name:Preprod" || got[0].Status != Pending {
		t.Errorf("kuma 1.x : %+v", got)
	}
}

func TestUneLigneMalFormeeEstRefusee(t *testing.T) {
	for _, line := range []string{`monitor_status{monitor_name="x"`, `monitor_status{monitor_name=x} 1`, `monitor_status`} {
		if _, err := ParsePrometheus(strings.NewReader(line)); err == nil {
			t.Errorf("%q accepte", line)
		}
	}
}

func TestLAdresseDesMesures(t *testing.T) {
	for in, want := range map[string]string{
		"https://kuma.example":          "https://kuma.example/metrics",
		"https://kuma.example/":         "https://kuma.example/metrics",
		" https://kuma.example/metrics": "https://kuma.example/metrics",
		"https://example.com/kuma":      "https://example.com/kuma/metrics",
	} {
		if got := MetricsURL(in); got != want {
			t.Errorf("%q : %q, attendu %q", in, got, want)
		}
	}
}

func TestLaReleveSAuthentifieAvecLaCle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if r.URL.Path != "/metrics" || !ok || user != "" || pass != "uk2_cle" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(kuma2))
	}))
	defer srv.Close()

	got, err := (&Kuma{BaseURL: srv.URL, APIKey: "uk2_cle"}).ListMonitors(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("releve : %v, %d sites", err, len(got))
	}
	if _, err := (&Kuma{BaseURL: srv.URL, APIKey: "mauvaise"}).ListMonitors(context.Background()); err != ErrUnauthorized {
		t.Errorf("mauvaise cle : %v", err)
	}
}

func TestUnePageQuiNEstPasKumaEstSignalee(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("process_cpu_seconds_total 1\n"))
	}))
	defer srv.Close()

	if _, err := (&Kuma{BaseURL: srv.URL}).ListMonitors(context.Background()); err == nil {
		t.Error("une page sans mesure de site passe pour Kuma")
	}
}

func TestLeWebhookDeKuma(t *testing.T) {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	body := `{"heartbeat":{"monitorID":3,"status":0,"time":"2026-10-12 08:41:07.120","msg":"timeout of 48000ms exceeded","ping":null},
	          "monitor":{"id":3,"name":"Vitrine","url":"https://atelier.example"},"msg":"[Vitrine] [🔴 Down] timeout"}`
	ev, ok, err := ParseKumaEvent([]byte(body), now)
	if err != nil || !ok {
		t.Fatalf("notification : %v, %v", ok, err)
	}
	if ev.Status != Down || ev.Message != "timeout of 48000ms exceeded" || len(ev.Keys) != 2 || ev.Keys[0] != "id:3" || ev.Keys[1] != "name:Vitrine" {
		t.Errorf("evenement : %+v", ev)
	}
	if want := time.Date(2026, 10, 12, 8, 41, 7, 120e6, time.UTC); !ev.At.Equal(want) {
		t.Errorf("heure : %v, attendu %v", ev.At, want)
	}

	// L'essai d'une notification depuis Kuma : ni battement, ni site.
	if _, ok, err := ParseKumaEvent([]byte(`{"heartbeat":null,"monitor":null,"msg":"Uptime Kuma Testing"}`), now); ok || err != nil {
		t.Errorf("essai : %v, %v", ok, err)
	}
	if _, _, err := ParseKumaEvent([]byte(`pas du json`), now); err == nil {
		t.Error("corps illisible accepte")
	}
}
